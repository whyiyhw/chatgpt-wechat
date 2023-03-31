package wecom

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/xen0n/go-workwx"
)

var (
	Token    string
	RestPort int
)

func SendToUser(agentID int64, userID string, msg string, corpID string, corpSecret string) {

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
	}
	if msg.MsgType == workwx.MessageTypeImage {
		p, ok := msg.Image()
		if ok {
			realLogic("openai", "#image:"+p.GetPicURL(), msg.FromUserID, msg.AgentID)
		}
	}

	return nil
}

func XmlServe(pToken, pEncodingAESKey, accessSecret string, accessExpire int64, port, restPort int) {
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
