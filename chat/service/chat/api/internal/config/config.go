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

	SystemVersion string `json:",optional,default=v0.6.5.0"`

	// jwt 配置
	Auth struct {
		AccessSecret string `json:",optional,default=13450cd8841c0f0"`
		AccessExpire int64  `json:",optional,default=25920000"`
	}

	// 企业微信，配置信息
	WeCom struct {
		Port                  int `json:",optional,default=8887"`
		CorpID                string
		DefaultAgentSecret    string `json:",optional"`
		CustomerServiceSecret string `json:",optional"`
		CorpSecret            string `json:",optional"`
		Model                 string `json:",optional,default=gpt-3.5-turbo"`
		BasePrompt            string `json:",optional,default=你是ChatGPT，一个由OpenAI训练的大型语言模型，你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。"`
		Welcome               string `json:",optional,default=您好！我是ChatGPT，一个由OpenAI训练的大型语言模型，我可以回答您的问题和进行交流。请告诉我您需要了解些什么，我会尽力为您提供答案。\n\n发送#help查看更多功能"`
		Token                 string `json:",optional"`
		EncodingAESKey        string `json:",optional"`
		MultipleApplication   []struct {
			AgentID     int64
			AgentSecret string
			Model       string `json:",optional,default=gpt-3.5-turbo"`
			BasePrompt  string `json:",optional,default=你是ChatGPT，一个由OpenAI训练的大型语言模型，你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。"`
			Welcome     string `json:",optional,default=您好！我是ChatGPT，一个由OpenAI训练的大型语言模型，我可以回答您的问题和进行交流。请告诉我您需要了解些什么，我会尽力为您提供答案。\n\n发送#help查看更多功能"`
			GroupEnable bool   `json:",optional,default=false"`
			GroupName   string `json:",optional,default=ChatGPT应用内部交流群"`
			GroupChatID string `json:",optional,default=ChatGPT202304021958"`
		} `json:",optional"`
	}

	// openai 配置
	OpenAi struct {
		Key         string
		Host        string  `json:",optional,default=https://api.openai.com"`
		Origin      string  `json:",optional,default=open_ai"`
		Engine      string  `json:",optional,default="`
		MaxToken    int     `json:",optional,default=2000"`
		TotalToken  int     `json:",optional,default=3900"`
		Temperature float32 `json:",optional,default=0.8"`
	}

	// http proxy 设置
	Proxy struct {
		Enable  bool   `json:",optional,default=false"`
		Socket5 string `json:",optional,default="`
		Http    string `json:",optional,default="`
		Auth    struct {
			Username string `json:",optional,default="`
			Password string `json:",optional,default="`
		} `json:",optional"`
	} `json:",optional"`

	// ocr 配置
	OCR struct {
		Company string `json:",optional"`
		AliYun  struct {
			AccessKeyId     string
			AccessKeySecret string
		} `json:",optional"`
	} `json:",optional"`

	// embeddings 配置
	Embeddings struct {
		Enable bool `json:",optional,default=false"`
		Mlvus  struct {
			Host     string   `json:",optional,default=127.0.0.1:19530"`
			Keywords []string `json:",optional"`
		} `json:",optional"`
	}

	// 流式传输 加快响应速度
	Response struct {
		Stream bool `json:",optional,default=true"`
	}

	// Plugins 配置
	Plugins struct {
		Enable bool `json:",optional,default=false"`
		Debug  bool `json:",optional,default=false"`
		List   []struct {
			Enable       bool   `json:",optional,default=true"`
			NameForHuman string `json:",optional"`
			NameForModel string `json:",optional"`
			DescForHuman string `json:",optional"`
			DescModel    string `json:",optional"`
			Auth         struct {
				Type string `json:",optional"`
			} `json:",optional"`
			API struct {
				URL string `json:",optional"`
			}
		} `json:",optional"`
	}

	// 语音解析与合成 后续可能会使用
	Speaker struct {
		Company string `json:",optional,default=openai"`
		AliYun  struct {
			AccessKeyId     string
			AccessKeySecret string
			AppKey          string
		} `json:",optional"`
	} `json:",optional"`

	// 作图相关配置，目前支持 StableDiffusion openai
	Draw struct {
		Enable          bool   `json:",optional,default=false"`
		Company         string `json:",optional,default=openai"`
		ZhCn2Prompt     string `json:",optional,default="`
		StableDiffusion struct {
			Host string `json:",optional,default="`
			Auth struct {
				Username string
				Password string
			}
		} `json:",optional"`
		OpenAi struct {
			Key   string `json:",optional"`
			Host  string `json:",optional,default=https://api.openai.com"`
			Proxy string `json:",optional,default="`
			Auth  struct {
				Username string `json:",optional,default="`
				Password string `json:",optional,default="`
			} `json:",optional"`
		}
	} `json:",optional"`

	// 会话相关控制
	Session struct {
		TimeOut int64 `json:",optional,default=-1"`
	} `json:",optional"`
}
