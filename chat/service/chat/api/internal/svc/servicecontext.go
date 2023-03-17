package svc

import (
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"

	"chat/service/chat/api/internal/config"
	"chat/service/chat/api/internal/middleware"
	"chat/service/chat/model"
)

type ServiceContext struct {
	Config            config.Config
	UserModel         model.UserModel
	ChatModel         model.ChatModel
	ChatConfigModel   model.ChatConfigModel
	PromptConfigModel model.PromptConfigModel
	AccessLog         rest.Middleware
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	return &ServiceContext{
		Config:            c,
		UserModel:         model.NewUserModel(conn, c.RedisCache),
		ChatModel:         model.NewChatModel(conn, c.RedisCache),
		ChatConfigModel:   model.NewChatConfigModel(conn, c.RedisCache),
		PromptConfigModel: model.NewPromptConfigModel(conn, c.RedisCache),
		AccessLog:         middleware.NewAccessLogMiddleware().Handle,
	}
}
