package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"chat/service/websocket/cmd/bucket"
	"chat/service/websocket/cmd/crontab"
	"chat/service/websocket/cmd/response"
	"chat/service/websocket/cmd/routers"

	"github.com/whyiyhw/gws"
)

func main() {

	server := new(gws.Server)

	// 未认证的bucket 默认初始化10个空闲map
	unAuthBuckets := bucket.NewMapBucket(10)

	// 已认证的bucket 默认初始化10个空闲map
	authBuckets := bucket.NewMapBucket(10)

	// 设置 auth 函数
	authRoute := routers.AuthFunc{
		GetIdByToken: func(token string) (int64, error) {
			url := "http://chat-api:8888/api/user/detail"
			method := "GET"
			client := &http.Client{
				Timeout: 3 * time.Second,
			}
			req, err := http.NewRequest(method, url, nil)

			if err != nil {
				fmt.Println(err)
				return 0, err
			}
			req.Header.Add("Authorization", "Bearer "+token)

			res, err := client.Do(req)
			if err != nil {
				return 0, err
			}
			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {
					fmt.Println(err)
				}
			}(res.Body)

			body, err := io.ReadAll(res.Body)
			if err != nil {
				fmt.Println(err)
				return 0, err
			}
			fmt.Println(string(body))
			type jsonDataType struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
				Data struct {
					ID    int64  `json:"id"`
					Email string `json:"email"`
					Name  string `json:"name"`
				} `json:"data"`
			}
			userData := jsonDataType{}
			err = json.Unmarshal(body, &userData)
			if err != nil {
				return 0, err
			}

			return userData.Data.ID, nil
		},
	}

	// 接收消息事件
	server.OnMessage = func(conn *gws.Conn, fd int, message string, err error) {

		// 未认证的 bucket 里面
		if unAuthBuckets.Exist(fd) {
			authErr := routers.ParseAndAuth(message, fd, conn, unAuthBuckets, authBuckets, authRoute) // 接收后，解析对应连接发过来的消息
			if authErr != nil {
				_, _ = conn.Write(response.Error(501, authErr.Error()))
				return
			}
			_, _ = conn.Write(response.Success(201))
			return
		}

		// 已认证的 bucket 里面
		if authBuckets.Exist(fd) {
			heartbeatErr := routers.ParseAndExec(message, fd, authBuckets) // 接收后，解析对应连接发过来的消息
			if heartbeatErr != nil {
				_, _ = conn.Write(response.Error(502, heartbeatErr.Error()))
				return
			}
			_, _ = conn.Write(response.Success(202))
			return
		}

		// 其它情况,不在维护范围内，不处理
	}

	// 连接事件
	server.OnOpen = func(conn *gws.Conn, fd int) {
		unAuthBuckets.Set(fd, bucket.New(conn, fd)) // 连接后，把 fd 存入未授权的 buckets
	}

	// 关闭事件
	server.OnClose = func(conn *gws.Conn, fd int) {
		// 解除 授权/未授权 bucket 关联
		unAuthBuckets.Delete(fd)
		authBuckets.Delete(fd)
	}

	// http处理事件
	server.OnHttp = append(server.OnHttp, routers.GetHttpHandleList(authBuckets, unAuthBuckets)...)

	// 定时器-清理 未授权/未响应 连接
	go crontab.TimingClearConn(unAuthBuckets, authBuckets)

	// 启动服务
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
