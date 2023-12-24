package svc

import (
	"github.com/zeromicro/go-zero/rest"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"chat/service/chat/api/internal/config"
	"chat/service/chat/api/internal/middleware"
	"chat/service/chat/dao"
)

type ServiceContext struct {
	Config            config.Config
	DbEngin           *gorm.DB
	UserModel         *dao.Query
	ChatModel         *dao.Query
	ChatConfigModel   *dao.Query
	PromptConfigModel *dao.Query
	BotModel          *dao.Query
	BotsPromptModel   *dao.Query
	AccessLog         rest.Middleware
}

func NewServiceContext(c config.Config) *ServiceContext {
	//启动Gorm支持
	db, err := gorm.Open(mysql.Open(c.Mysql.DataSource), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})

	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config:            c,
		DbEngin:           db,
		UserModel:         dao.Use(db),
		ChatModel:         dao.Use(db),
		ChatConfigModel:   dao.Use(db),
		PromptConfigModel: dao.Use(db),
		BotModel:          dao.Use(db),
		BotsPromptModel:   dao.Use(db),
		AccessLog:         middleware.NewAccessLogMiddleware().Handle,
	}
}
