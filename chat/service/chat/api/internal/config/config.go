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

	Log struct {
		Stat bool `json:"stat,optional,default=false"`
	}

	// jwt 配置
	Auth struct {
		AccessSecret string `json:",optional,default=13450cd8841c0f0"`
		AccessExpire int64  `json:",optional,default=25920000"`
	}

	// 企业微信，配置信息
	WeCom struct {
		CorpID              string
		CorpSecret          string
		Model               string `json:",optional,default=gpt-3.5-turbo-0301"`
		BasePrompt          string `json:",optional,default=你是ChatGPT，一个由OpenAI训练的大型语言模型，你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。"`
		Welcome             string `json:",optional,default=您好！我是ChatGPT，一个由OpenAI训练的大型语言模型，我可以回答您的问题和进行交流。请告诉我您需要了解些什么，我会尽力为您提供答案。\n\n发送#help查看更多功能"`
		Token               string `json:",optional"`
		EncodingAESKey      string `json:",optional"`
		MultipleApplication []struct {
			AgentID     int64
			AgentSecret string
			Model       string `json:",optional,default=gpt-3.5-turbo-0301"`
			BasePrompt  string `json:",optional,default=你是ChatGPT，一个由OpenAI训练的大型语言模型，你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。"`
			Welcome     string `json:",optional,default=您好！我是ChatGPT，一个由OpenAI训练的大型语言模型，我可以回答您的问题和进行交流。请告诉我您需要了解些什么，我会尽力为您提供答案。\n\n发送#help查看更多功能"`
		} `json:",optional"`
	}

	// openai 配置
	OpenAi struct {
		Key  string
		Host string `json:"host,optional,default=https://api.openai.com"`
	}

	// http proxy 设置
	Proxy struct {
		Enable  bool   `json:",optional,default=false"`
		Socket5 string `json:",optional,default=127.0.0.1:1080"`
	} `json:",optional"`

	// ocr 配置
	OCR struct {
		Company string `json:",optional"`
		AliYun  struct {
			AccessKeyId     string
			AccessKeySecret string
		} `json:",optional"`
	} `json:",optional"`
}
