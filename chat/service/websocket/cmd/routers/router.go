package routers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/whyiyhw/gws"

	"chat/service/websocket/cmd/bucket"
)

type Route struct {
	Action string
}

type Auth struct {
	Action string
	Token  string
}

// AuthFunc 认证函数
type AuthFunc struct {
	GetIdByToken func(token string) (int64, error)
}

const ActionAuth = "auth"
const ActionHeartbeat = "heartbeat"

// ParseAndExec 解析并执行
func ParseAndExec(s string, uid int, m *bucket.MapBucket) error {
	r := Route{}
	err := json.Unmarshal([]byte(s), &r)

	if err != nil {
		fmt.Printf("client:%d send msg:%s", uid, s)
		return fmt.Errorf("heartbeat 信息传入错误~")
	}

	if r.Action == ActionHeartbeat {
		m.Heartbeat(uid)
		return nil
	}

	// 未知行为~
	return fmt.Errorf("heartbeat 信息传入错误~")
}

// ParseAndAuth 解析并认证
func ParseAndAuth(s string, fd int, c *gws.Conn, unAuth, auth *bucket.MapBucket, authFc AuthFunc) error {
	r := Auth{}
	fmt.Printf("client:%d send msg:%s", fd, s)
	err := json.Unmarshal([]byte(s), &r)

	if err != nil {
		fmt.Printf("client:%d send msg:%s", fd, s)
		return fmt.Errorf("auth fail param error")
	}

	if r.Action == ActionAuth && r.Token != "" {

		id, err := authFc.GetIdByToken(r.Token)
		if err != nil {
			fmt.Printf("client:%d send msg:%s", fd, s)
			return fmt.Errorf("auth fail token error")
		}
		fmt.Println(id, "用户认证成功~")

		// success && set it to auth bucket
		auth.Set(fd, bucket.New(c, int(id)))
		unAuth.Delete(fd)
		return nil
	}

	// 成功
	return fmt.Errorf("auth 信息传入错误~")
}

// GetHttpHandleList 获取 http handle 列表
func GetHttpHandleList(a, u *bucket.MapBucket) []*gws.HttpHandler {

	// http 获取状态
	statusHandler := new(gws.HttpHandler)
	statusHandler.Path = "/ws/status"
	statusHandler.DealFunc = func(w http.ResponseWriter, r *http.Request) {
		// 获取当前维护bucket的状态
		data := make(map[string]interface{})
		response := make(map[string]interface{})
		response["code"] = 200
		response["msg"] = "success"
		data["auth_len"] = a.Len()
		data["un_auth_len"] = u.Len()
		data["auth_msg"] = a.EachStatus()
		data["un_auth_msg"] = u.EachStatus()
		response["data"] = data
		resData, _ := json.Marshal(response)
		_, _ = io.WriteString(w, string(resData))
	}

	// http 发送数据
	sendHandler := new(gws.HttpHandler)
	sendHandler.Path = "/ws/send"
	sendHandler.DealFunc = func(w http.ResponseWriter, r *http.Request) {

		body, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Printf("read body err, %v\n", err)
			return
		}

		type RequestData struct {
			ToUser string      `json:"to_user"`
			Msg    string      `json:"msg"`
			Data   interface{} `json:"data"`
		}
		var b RequestData
		if err = json.Unmarshal(body, &b); err != nil {
			fmt.Printf("Unmarshal err, %v\n", err)
			return
		}

		var ids []int
		if b.ToUser == "all" {
			ids = make([]int, 0)
		} else {
			if strings.ContainsAny(b.ToUser, ",") {
				for _, s := range strings.Split(b.ToUser, ",") {
					i, _ := strconv.Atoi(s)
					ids = append(ids, i)
				}
			} else {
				i, _ := strconv.Atoi(b.ToUser)
				ids = append(ids, i)
			}
		}

		response := make(map[string]interface{})
		response["code"] = 200
		response["msg"] = b.Msg
		response["data"] = b.Data
		sendData, _ := json.Marshal(response)

		ok := a.EachSendMsg(string(sendData), ids)

		response["msg"] = "成功给" + strconv.Itoa(ok) + "个用户发送消息"
		response["data"] = []int{}
		resData, _ := json.Marshal(response)
		// 获取当前维护bucket的状态
		_, _ = io.WriteString(w, string(resData))
	}

	return append([]*gws.HttpHandler{}, sendHandler, statusHandler)
}
