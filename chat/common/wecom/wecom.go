package wecom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"chat/common/redis"

	"github.com/golang-jwt/jwt/v4"
	"github.com/whyiyhw/go-workwx"
)

var (
	Token                 string
	RestPort              int
	CorpID                string
	CustomerServiceSecret string
)

func SendToWeComUser(agentID int64, userID string, msg string, corpID string, corpSecret string) {

	go func() {
		// 然后把数据 发给微信用户
		app := workwx.New(corpID).WithApp(corpSecret, agentID)

		recipient := workwx.Recipient{
			UserIDs: []string{userID},
		}
		rs := []rune(msg)

		//当 msg 大于 1000个字符 的时候切割发送
		//3. 企业微信会话发送文字的字符上限
		//（1）会话消息目前支持2000（不确定）？字符，1个汉字=2字符，1个英文、符号=1个字符。
		if len(rs) > 850 {
			msgs := splitMsg(rs, 850)
			for _, v := range msgs {
				_ = app.SendTextMessage(&recipient, v, false)
			}
			return
		}

		_ = app.SendTextMessage(&recipient, msg, false)
	}()
}

func splitMsg(rs []rune, i int) []string {
	var msgs []string
	for len(rs) > i {
		msgs = append(msgs, string(rs[:i]))
		rs = rs[i:]
	}
	msgs = append(msgs, string(rs))
	return msgs
}

func DealUserLastMessageByToken(token, openKfID string) {
	app := workwx.New(CorpID).WithApp(CustomerServiceSecret, 0)
	cacheKey := fmt.Sprintf(redis.CursorCacheKey, openKfID)
	cursor, _ := redis.Rdb.Get(context.Background(), cacheKey).Result()

	msg, err := app.GetKFSyncMsg(cursor, token, openKfID, 20, 0)
	if err != nil {
		fmt.Println("客服消息 获取body err:", err)
		return
	}

	fmt.Println("客服消息 获取 message success", msg.NextCursor, msg.MsgList)

	_, _ = redis.Rdb.Set(context.Background(), cacheKey, msg.NextCursor, 24*30*time.Hour).Result()
	for _, v := range msg.MsgList {
		if v.Msgtype == "text" && v.Origin == 3 {
			CustomerCallLogic(v.ExternalUserid, v.OpenKfid, v.Msgid, v.Text.Content)
		}
	}
}

// SendCustomerChatMessage 发送给客户消息
func SendCustomerChatMessage(openKfID, customerID, msg string) {

	go func() {
		// 然后把数据 发给微信用户
		app := workwx.New(CorpID).WithApp(CustomerServiceSecret, 0)

		recipient := workwx.Recipient{
			UserIDs:  []string{customerID},
			OpenKfID: openKfID,
		}
		rs := []rune(msg)

		//当 msg 大于 1000个字符 的时候切割发送
		//3. 企业微信会话发送文字的字符上限
		//（1）会话消息目前支持2000（不确定）？字符，1个汉字=2字符，1个英文、符号=1个字符。
		if len(rs) > 850 {
			msgs := splitMsg(rs, 850)
			for _, v := range msgs {
				err := app.SendTextMessage(&recipient, v, false)
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

func XmlServe(corpID, pToken, pEncodingAESKey, customerServiceSecret, accessSecret string, accessExpire int64, port, restPort int) {
	pAddr := fmt.Sprintf("[::]:%d", port)

	// build a json web token
	iat := time.Now().Unix()
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + accessExpire
	claims["iat"] = iat
	claims["userId"] = 1
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims
	Token, _ = token.SignedString([]byte(accessSecret))
	RestPort = restPort
	CorpID = corpID
	CustomerServiceSecret = customerServiceSecret

	hh, err := workwx.NewHTTPHandler(pToken, pEncodingAESKey, dummyRxMessageHandler{})
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
	url := fmt.Sprintf("http://localhost:%d/api/msg/push", RestPort)
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

	b, _ := json.Marshal(r)

	payload := strings.NewReader(string(b))

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+Token)

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(res.Body)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(body))
}

func CustomerCallLogic(CustomerID, OpenKfID, MsgID, Msg string) {
	url := fmt.Sprintf("http://localhost:%d/api/msg/customer/push", RestPort)
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
		fmt.Println(err)
		return
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+Token)

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(res.Body)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(body))
}
