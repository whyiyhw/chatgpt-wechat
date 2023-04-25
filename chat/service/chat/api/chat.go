package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"

	"chat/common/accesslog"
	"chat/common/redis"
	"chat/common/response"
	"chat/common/wecom"
	"chat/common/xerr"
	"chat/service/chat/api/internal/config"
	"chat/service/chat/api/internal/handler"
	"chat/service/chat/api/internal/svc"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/chat-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf,
		rest.WithUnauthorizedCallback(func(w http.ResponseWriter, r *http.Request, err error) {
			bodyByte, _ := io.ReadAll(r.Body)
			accesslog.ToLog(r, bodyByte, -1)
			response.Response(r, w, nil, errors.Wrapf(xerr.NewErrCode(xerr.UNAUTHORIZED), "鉴权失败 %v", err))
			return
		}),
		rest.WithNotFoundHandler(&NotFoundHandler{}),
		rest.WithNotAllowedHandler(&MethodNotMatchHandler{}),
	)
	defer server.Stop()

	redis.Init(c.RedisCache[0].Host, c.RedisCache[0].Pass)
	defer redis.Close()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)
	wecom.WeCom.RestPort = c.RestConf.Port
	wecom.WeCom.Port = c.WeCom.Port
	wecom.WeCom.DefaultAgentSecret = c.WeCom.DefaultAgentSecret
	wecom.WeCom.CorpID = c.WeCom.CorpID
	wecom.WeCom.CustomerServiceSecret = c.WeCom.CustomerServiceSecret
	wecom.WeCom.Token = c.WeCom.Token
	wecom.WeCom.EncodingAESKey = c.WeCom.EncodingAESKey
	wecom.WeCom.Auth.AccessSecret = c.Auth.AccessSecret
	wecom.WeCom.Auth.AccessExpire = c.Auth.AccessExpire
	for _, v := range c.WeCom.MultipleApplication {
		wecom.WeCom.MultipleApplication = append(wecom.WeCom.MultipleApplication, wecom.Application{
			AgentID:     v.AgentID,
			AgentSecret: v.AgentSecret,
		})
	}

	go wecom.XmlServe()

	if len(c.WeCom.MultipleApplication) > 0 {
		for _, v := range c.WeCom.MultipleApplication {
			if v.GroupEnable {
				fmt.Println("初始化群聊", v.GroupName, v.GroupChatID, c.WeCom.CorpID, v.AgentSecret, v.AgentID)
				go wecom.InitGroup(v.GroupName, v.GroupChatID, v.AgentSecret, v.AgentID)
			}
		}
	}

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}

type NotFoundHandler struct{}

func (h *NotFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bodyByte, _ := io.ReadAll(r.Body)
	accesslog.ToLog(r, bodyByte, -1)
	response.Response(r, w, nil, errors.Wrapf(xerr.NewErrCode(xerr.RouteNotFound), "接口不存在"))
	return
}

type MethodNotMatchHandler struct{}

func (h *MethodNotMatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bodyByte, _ := io.ReadAll(r.Body)
	accesslog.ToLog(r, bodyByte, -1)
	response.Response(r, w, nil, errors.Wrapf(xerr.NewErrCode(xerr.RouteNotMatch), "请求方式错误"))
	return
}
