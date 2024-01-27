package http2websocket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var MsgTypeError MessageType = "error"
var MsgTypeTxt MessageType = "txt"
var MsgTypeStop MessageType = "stop"

type MessageType string

// SendData 发送给用户数据
type SendData struct {
	MsgID       string      `json:"msg_id"`
	Msg         string      `json:"msg"`
	MsgType     MessageType `json:"msg_type"`
	MsgToUserID string      `json:"msg_to_user_id"`
}

// RealSendMsg 实际上发送给websocket的数据
type RealSendMsg struct {
	Msg    string `json:"msg"`
	ToUser string `json:"to_user"`
	Data   struct {
		MessageID string      `json:"message_id"`
		MsgType   MessageType `json:"msg_type"`
		Content   string      `json:"content"`
	} `json:"data"`
}

// Http2websocket 给(全体/指定)用户推送消息
func Http2websocket(send SendData) {

	newSendMsg := RealSendMsg{
		Msg:    send.Msg,
		ToUser: send.MsgToUserID,
		Data: struct {
			MessageID string      `json:"message_id"`
			MsgType   MessageType `json:"msg_type"`
			Content   string      `json:"content"`
		}{
			MessageID: send.MsgID,
			MsgType:   send.MsgType,
			Content:   send.Msg,
		},
	}

	url := "http://chat-websocket:9501/ws/send"
	method := "POST"

	payload, err := json.Marshal(newSendMsg)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(body))
	return
}
