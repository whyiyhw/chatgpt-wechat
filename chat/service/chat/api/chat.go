package main

import (
	"chat/common/accesslog"
	"chat/common/response"
	"chat/common/xerr"
	"chat/service/chat/api/internal/config"
	"chat/service/chat/api/internal/handler"
	"chat/service/chat/api/internal/svc"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"io"
	"net/http"
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
	)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
