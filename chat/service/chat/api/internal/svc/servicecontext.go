package svc

import (
	"chat/common/aibackend"
	_ "chat/common/aibackend/dify" // 自注册 "dify" Factory 到 aibackend.Registry
	"chat/service/chat/api/internal/config"
	"chat/service/chat/api/internal/middleware"
	"chat/service/chat/dao"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	Knowledge         *dao.Query
	AccessLog         rest.Middleware

	// AIBackend 路由器：v1.1 起统一通过它访问各 AI 后端。
	// 未配 AIBackend 块时是一个空 Router（Default()/Get() 返回 nil），
	// 调用方需 nil-check 后回落到旧的 ModelProvider 分支。
	AIBackend *aibackend.Router
}

func NewServiceContext(c config.Config) *ServiceContext {
	//启动Gorm支持
	//db, err := gorm.Open(mysql.Open(c.Mysql.DataSource), &gorm.Config{
	//	DisableForeignKeyConstraintWhenMigrating: true,
	//	SkipDefaultTransaction:                   true,
	//	Logger:                                   logger.Default.LogMode(logger.Info),
	//})

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  c.PGSql.DataSource,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		Logger:                                   logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		panic(err)
	}

	router, err := aibackend.NewRouter(c.AIBackend)
	if err != nil {
		// 配置错误在启动期 fail-fast，避免运行时才发现。
		panic(err)
	}
	if names := router.Names(); len(names) > 0 {
		logx.Infof("AIBackend ready: default=%q rollout=%d%% backends=%v",
			c.AIBackend.Default, router.RolloutPercent(), names)
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
		Knowledge:         dao.Use(db),
		AccessLog:         middleware.NewAccessLogMiddleware().Handle,
		AIBackend:         router,
	}
}
