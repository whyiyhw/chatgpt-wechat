package wecom

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"chat/common/redis"

	"github.com/golang-jwt/jwt/v4"
	"github.com/whyiyhw/go-workwx"
	"github.com/zeromicro/go-zero/core/logx"
)

var (
	Token string

	WeCom struct {
		Port                  int
		RestPort              int
		CorpID                string
		DefaultAgentSecret    string
		CustomerServiceSecret string
		Token                 string
		EncodingAESKey        string
		MultipleApplication   []Application
		Auth                  struct {
			AccessSecret string
			AccessExpire int64
		}
	}
)

type Application struct {
	AgentID     int64
	AgentSecret string
}

// SendToWeComUser 发送应用消息给用户
func SendToWeComUser(agentID int64, userID, msg, corpSecret string, images ...string) {

	if len(images) > 0 {
		go func() {
			app := workwx.New(WeCom.CorpID).WithApp(corpSecret, agentID)
			recipient := workwx.Recipient{
				UserIDs: []string{userID},
			}
			for _, path := range images {
				//生成唯一的文件名
				fileName := uuid.New().String() + ".png"
				buf, _ := os.ReadFile(path) //读取文件
				media, err := workwx.NewMediaFromBuffer(fileName, buf)
				if err != nil {
					logx.Error("应用图片消息-读取文件失败 err:", err)
					//发送给用户失败信息
					err = app.SendTextMessage(&recipient, "发送图片失败", false)
					return
				}
				// 上传图片
				mediaID, err := app.UploadTempImageMedia(media)
				if err != nil {
					logx.Error("应用图片消息-上传图片失败 err:", err)
					//发送给用户失败信息
					err = app.SendTextMessage(&recipient, "发送图片失败", false)
					return
				}
				err = app.SendImageMessage(&recipient, mediaID.MediaID, false)
				if err != nil {
					logx.Error("应用图片消息-发送失败 err:", err)
				}
			}
		}()
		return
	}

	go func() {
		app := workwx.New(WeCom.CorpID).WithApp(corpSecret, agentID)
		recipient := workwx.Recipient{
			UserIDs: []string{userID},
		}
		rs := []rune(msg)

		//当 msg 大于 850 个字符 的时候切割发送，避免被企业微信吞掉
		if len(rs) > 850 {
			messages := splitMsg(rs, 850)
			for _, message := range messages {
				err := app.SendTextMessage(&recipient, message, false)
				if err != nil {
					logx.Error("应用消息-发送失败 err:", err)
				}
			}
			return
		}

		err := app.SendTextMessage(&recipient, msg, false)
		if err != nil {
			logx.Error("应用消息-发送失败 err:", err)
		}
	}()
}

// splitMsg 切割多字节字符串
func splitMsg(rs []rune, i int) []string {
	var msgList []string
	for len(rs) > i {
		msgList = append(msgList, string(rs[:i]))
		rs = rs[i:]
	}
	msgList = append(msgList, string(rs))
	return msgList
}

func DealUserLastMessageByToken(token, openKfID string) {
	app := workwx.New(WeCom.CorpID).WithApp(WeCom.CustomerServiceSecret, 0)
	cacheKey := fmt.Sprintf(redis.CursorCacheKey, openKfID)
	cursor, _ := redis.Rdb.Get(context.Background(), cacheKey).Result()

	msg, err := app.GetKFSyncMsg(cursor, token, openKfID, 500, 0)
	if err != nil {
		fmt.Println("客服消息 获取body err:", err)
		return
	}

	fmt.Println("客服消息 获取 message success. NextCursor:", msg.NextCursor)

	_, _ = redis.Rdb.Set(context.Background(), cacheKey, msg.NextCursor, 24*30*time.Hour).Result()
	for _, v := range msg.MsgList {
		// 仅处理发送时间在5分钟内的消息
		if v.SendTime < time.Now().Unix()-300 {
			logx.Info("客服消息-消息过期", v.SendTime, time.Now().Unix()-300)
			continue
		}
		if v.Msgtype == "text" && v.Origin == 3 {
			CustomerCallLogic(v.ExternalUserid, v.OpenKfid, v.Msgid, v.Text.Content)
		}
		if v.Msgtype == "voice" && v.Origin == 3 {
			filePath, err := DealCustomerVoiceMessageByMediaID(v.Voice.MediaId)
			if err != nil {
				logx.Info("音频文件读取失败", v.Voice.MediaId)
				CustomerCallLogic(v.ExternalUserid, v.OpenKfid, v.Msgid, "音频文件读取失败:"+err.Error())
			} else {
				CustomerCallLogic(v.ExternalUserid, v.OpenKfid, v.Msgid, "#voice:"+filePath)
			}
		}
	}
}

// SendCustomerChatMessage 发送客服消息
func SendCustomerChatMessage(openKfID, customerID, msg string) {

	go func() {
		// 然后把数据 发给微信用户
		app := workwx.New(WeCom.CorpID).WithApp(WeCom.CustomerServiceSecret, 0)

		recipient := workwx.Recipient{
			UserIDs:  []string{customerID},
			OpenKfID: openKfID,
		}
		rs := []rune(msg)

		//当 msg 大于 850 个字符 的时候切割发送，避免被企业微信吞掉
		if len(rs) > 850 {
			messages := splitMsg(rs, 850)
			for _, message := range messages {
				err := app.SendTextMessage(&recipient, message, false)
				if err != nil {
					fmt.Println("客服消息-发送失败 err:", err)
				}
			}
			return
		}

		err := app.SendTextMessage(&recipient, msg, false)
		if err != nil {
			fmt.Println("客服消息-发送失败 err:", err)
		}
	}()
}

type dummyRxMessageHandler struct{}

var _ workwx.RxMessageHandler = dummyRxMessageHandler{}

// OnIncomingMessage 一条消息到来时的回调。
func (dummyRxMessageHandler) OnIncomingMessage(msg *workwx.RxMessage) error {
	// You can do much more!
	fmt.Printf("incoming message: %s\n", msg)

	if msg.MsgType == workwx.MessageTypeText {
		message, ok := msg.Text()
		if ok {
			realLogic("openai", message.GetContent(), msg.FromUserID, msg.AgentID)
		}
	} else if msg.MsgType == workwx.MessageTypeVoice {
		message, ok := msg.Voice()
		if ok {
			filePath, err := DealUserVoiceMessageByMediaID(message.GetMediaID(), msg.AgentID)
			if err != nil {
				logx.Error("应用音频文件读取失败:", err.Error())
				realLogic("wecom", "音频文件读取失败:"+err.Error(), msg.FromUserID, msg.AgentID)
			} else {
				realLogic("openai", "#voice:"+filePath, msg.FromUserID, msg.AgentID)
			}
		}
	}

	if msg.MsgType == workwx.MessageTypeEvent {
		if string(msg.Event) == "enter_agent" {
			realLogic("openai", "#welcome", msg.FromUserID, msg.AgentID)
		}
		// 客服消息
		if msg.Event == workwx.EventTypeKFMsgOrEvent {
			fmt.Println("客服消息 -------------------------------------")
			p, ok := msg.EventTypeKFMsgOrEvent()
			if ok {
				fmt.Println("客服消息 Token:", p.Token, "OpenKfID:", p.OpenKfID)
				DealUserLastMessageByToken(p.Token, p.OpenKfID)
			}
		}
	}

	if msg.MsgType == workwx.MessageTypeImage {
		p, ok := msg.Image()
		if ok {
			realLogic("openai", "#image:"+p.GetPicURL(), msg.FromUserID, msg.AgentID)
		}
	}

	return nil
}

func XmlServe() {
	pAddr := fmt.Sprintf("[::]:%d", WeCom.Port)

	// build a json web token
	iat := time.Now().Unix()
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + WeCom.Auth.AccessExpire
	claims["iat"] = iat
	claims["userId"] = 1
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims
	Token, _ = token.SignedString([]byte(WeCom.Auth.AccessSecret))

	hh, err := workwx.NewHTTPHandler(WeCom.Token, WeCom.EncodingAESKey, dummyRxMessageHandler{})
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", hh)

	err = http.ListenAndServe(pAddr, mux)
	if err != nil {
		panic(err)
	}
}

func realLogic(channel, msg, userID string, agentID int64) {
	url := fmt.Sprintf("http://localhost:%d/api/msg/push", WeCom.RestPort)
	method := "POST"

	type ChatReq struct {
		Channel string `json:"channel"`
		MSG     string `json:"msg"`
		UserID  string `json:"user_id"`
		AgentID int64  `json:"agent_id"`
	}

	r := ChatReq{
		Channel: channel,
		MSG:     msg,
		UserID:  userID,
		AgentID: agentID,
	}

	b, err := json.Marshal(r)
	if err != nil {
		logx.Error("内部应用消息:请求参数json构造错误", err.Error())
	}

	payload := strings.NewReader(string(b))

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		logx.Error("内部应用消息:请求参数构造错误", err.Error())
		return
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+Token)

	res, err := client.Do(req)
	if err != nil {
		logx.Error("内部应用消息:请求错误", err.Error())
		return
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	_, err = io.ReadAll(res.Body)
	if err != nil {
		logx.Error("内部应用消息:响应读取错误", err.Error())
		return
	}
}

func CustomerCallLogic(CustomerID, OpenKfID, MsgID, Msg string) {
	url := fmt.Sprintf("http://localhost:%d/api/msg/customer/push", WeCom.RestPort)
	method := "POST"

	type ChatReq struct {
		MsgID      string `json:"msg_id"`
		Msg        string `json:"msg"`
		CustomerID string `json:"customer_id"`
		OpenKfID   string `json:"open_kf_id"`
	}

	r := ChatReq{
		OpenKfID:   OpenKfID,
		CustomerID: CustomerID,
		MsgID:      MsgID,
		Msg:        Msg,
	}

	b, _ := json.Marshal(r)

	payload := strings.NewReader(string(b))

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		logx.Error("客服消息:请求参数构造错误", err.Error())
		return
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+Token)

	res, err := client.Do(req)
	if err != nil {
		logx.Error("客服消息:请求错误", err.Error())
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	_, err = io.ReadAll(res.Body)
	if err != nil {
		logx.Error("客服消息：响应读取错误", err.Error())
		return
	}
}

// InitGroup 初始化群组
func InitGroup(name, chatID, corpSecret string, agentID int64) {
	// 然后把数据 发给微信用户
	app := workwx.New(WeCom.CorpID).WithApp(corpSecret, agentID)

	//  获取群聊会话
	appChat, err := app.GetAppchat(chatID)
	if err == nil && appChat != nil {
		fmt.Println("群聊已经存在，不需要创建")

		// 推送一条消息
		err = app.SendTextMessage(&workwx.Recipient{
			ChatID: appChat.ChatID,
		}, "应用重新部署完成", false)
		if err != nil {
			fmt.Println("应用消息发送失败 err:", err)
		}
		fmt.Println("应用消息发送成功")
		return
	}
	fmt.Println("群聊不存在，开始创建 err:", err)
	// 查询根部门的管理员与成员信息
	// 1. 获取根部门
	root, err := app.ListUsersByDeptID(1, true)
	if err != nil {
		fmt.Println("获取用户信息失败，群创建失败 err:", err)
		return
	}
	// 2. 获取管理员 与 成员
	owner := ""
	var userList []string
	for _, info := range root {
		if info.IsEnabled && info.Status == 1 {
			userList = append(userList, info.UserID)
		}
		for _, i2 := range info.Departments {
			if i2.DeptID == 1 && i2.IsLeader {
				owner = info.UserID
			}
		}
	}

	// 创建群聊
	chatIDInfo, err := app.CreateAppchat(&workwx.ChatInfo{
		ChatID:        chatID,
		Name:          name,
		OwnerUserID:   owner,
		MemberUserIDs: userList,
	})
	if err != nil {
		fmt.Println("创建群聊失败 err:", err)
		return
	}
	fmt.Println("创建群聊成功", chatIDInfo)
	// 推送一条消息
	err = app.SendTextMessage(&workwx.Recipient{
		ChatID: chatIDInfo,
	}, "应用初始化完成", false)
	if err != nil {
		fmt.Println("应用消息发送失败 err:", err)
	}
	fmt.Println("应用消息发送成功")
}

// DealUserVoiceMessageByMediaID 获取应用内语音消息
func DealUserVoiceMessageByMediaID(mediaID string, agentID int64) (string, error) {
	defaultAgentSecret := WeCom.DefaultAgentSecret
	for _, application := range WeCom.MultipleApplication {
		if application.AgentID == agentID {
			defaultAgentSecret = application.AgentSecret
		}
	}
	if defaultAgentSecret == "" {
		return "", fmt.Errorf("应用密钥不匹配")
	}
	app := workwx.New(WeCom.CorpID).WithApp(defaultAgentSecret, agentID)
	token := app.GetAccessToken()
	// https://qyapi.weixin.qq.com/cgi-bin/media/get?access_token=ACCESS_TOKEN&media_id=MEDIA_ID
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/get?access_token=%s&media_id=%s", token, mediaID)

	fmt.Println("req voice url:", url)

	filepath := fmt.Sprintf("/tmp/voice/%s", mediaID)
	err := DownloadFile("/tmp/voice", filepath, "amr", url)
	return filepath + ".mp3", err
}

func DownloadFile(fileDir, filepath, fileMime string, url string) error {

	// 判断目录是否存在
	_, err := os.Stat(fileDir)
	if err != nil {
		err := os.MkdirAll(fileDir, os.ModePerm)
		if err != nil {
			fmt.Println("mkdir err:", err)
			return err
		}
	}

	// Create the file
	out, err := os.Create(filepath + "." + fileMime)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			fmt.Println("file close err:", err)
		}
	}(out)

	// http download file
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("http get err:", err)
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("http close err:", err)
		}
	}(resp.Body)
	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		fmt.Println("下载失败:", resp.Status)
	}

	// 检查文件长度
	if resp.ContentLength <= 0 {
		fmt.Println("文件长度错误")
	} else {
		fmt.Println("文件长度", resp.ContentLength)
	}

	// 将内容写入文件
	w, err := io.Copy(out, resp.Body)
	if err != nil {
		fmt.Println("io copy err:", err)
		return err
	}
	fmt.Println("文件大小:", w)

	fmt.Println("/bin/ffmpeg", "-i", filepath+"."+fileMime, filepath+".mp3")
	// golang  arm 格式转 mp3
	cmd := exec.Command("/bin/ffmpeg", "-i", filepath+"."+fileMime, filepath+".mp3")

	err = cmd.Start()
	if err != nil {
		fmt.Println("cmd start err:", err)
		return err
	}
	err = cmd.Wait()
	if err != nil {
		fmt.Println("cmd start err:", err)
		return err
	}

	return nil
}

// DealCustomerVoiceMessageByMediaID 获取客服语音消息
func DealCustomerVoiceMessageByMediaID(mediaID string) (string, error) {
	defaultAgentSecret := WeCom.CustomerServiceSecret
	if defaultAgentSecret == "" {
		return "", fmt.Errorf("应用密钥不匹配")
	}
	app := workwx.New(WeCom.CorpID).WithApp(WeCom.CustomerServiceSecret, 0)
	token := app.GetAccessToken()
	// https://qyapi.weixin.qq.com/cgi-bin/media/get?access_token=ACCESS_TOKEN&media_id=MEDIA_ID
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/get?access_token=%s&media_id=%s", token, mediaID)

	fmt.Println("req voice url:", url)

	filepath := fmt.Sprintf("/tmp/voice/%s", mediaID)
	err := DownloadFile("/tmp/voice", filepath, "amr", url)
	return filepath + ".mp3", err
}
