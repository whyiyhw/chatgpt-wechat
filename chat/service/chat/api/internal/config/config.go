package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf

	Mysql struct {
		DataSource string
	}

	RedisCache cache.CacheConf
	Auth       struct {
		AccessSecret string
		AccessExpire int64
	}

	// 企业微信，配置信息
	WeCom struct {
		CorpID     string
		CorpSecret string

		MultipleApplication []struct {
			AgentID     int64
			AgentSecret string
		}
	}

	// openai 配置
	OpenAi struct {
		Key string
	}

	// http proxy 设置
	Proxy struct {
		Enable  bool
		Socket5 string
	}
}
