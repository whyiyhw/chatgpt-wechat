package sensecore

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

const SystemRole = "system"
const UserRole = "user"
const ModelRole = "assistant"

// 商汤 SenseCore 支持的模型

// 聊天模型

const SenseChat5_1202 = "SenseChat-5-1202"          // 通用自然语言 V5.5，32K上下文
const SenseChatTurbo_1202 = "SenseChat-Turbo-1202"  // 通用自然语言轻量 V5.5，32K上下文
const SenseChat5 = "SenseChat-5"                    // 通用 V5.5，128K上下文
const SenseChat = "SenseChat"                       // 通用 V4.0，4K上下文
const SenseChat32K = "SenseChat-32K"                // 通用 V4.0，32K上下文
const SenseChat128K = "SenseChat-128K"              // 通用 V4.0，128K上下文
const SenseChatTurbo = "SenseChat-Turbo"            // 通用 V5.0，32K上下文
const SenseChat5Cantonese = "SenseChat-5-Cantonese" // 粤语版本 V5.0，32K上下文

// 默认模型

const TextModel = SenseChat5_1202
const ChatModel = SenseChat5_1202
const ChatModel4 = SenseChat5_1202

// 嵌入模型

const EmbeddingModel = "nova-embedding-stable"

var TotalToken = 3900
var MaxToken = 2000
var Temperature = 0.8

const MimetypeTextPlain = "text/plain"

type ChatModelMessage struct {
	MessageId string      `json:"message_id"`
	Role      string      `json:"role"`
	Content   ChatContent `json:"content"`
}

func NewChatContent(data ...string) ChatContent {
	if len(data) == 0 {
		return ChatContent{}
	}
	if len(data) == 1 {
		return ChatContent{
			MIMEType: MimetypeTextPlain,
			Data:     data[0],
		}
	}
	return ChatContent{
		Data:     data[0],
		MIMEType: data[1],
	}
}

type ChatContent struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

type ChatClient struct {
	APIKey        string  `json:"api_key"`
	Origin        string  `json:"origin"`
	Engine        string  `json:"engine"`
	HttpProxy     string  `json:"http_proxy"`
	Socks5Proxy   string  `json:"socks5_proxy"`
	ProxyUserName string  `json:"proxy_user_name"`
	ProxyPassword string  `json:"proxy_password"`
	Model         string  `json:"model"`
	BaseHost      string  `json:"base_host"`
	MaxToken      int     `json:"max_token"`
	TotalToken    int     `json:"total_token"`
	Temperature   float32 `json:"temperature"`
	Debug         bool    `json:"debug"`
}

func NewChatClient(apiKey string) *ChatClient {
	return &ChatClient{
		APIKey:      apiKey,
		MaxToken:    MaxToken,
		TotalToken:  TotalToken,
		Temperature: float32(Temperature),
		BaseHost:    "https://api.sensenova.cn",
	}
}

// WithOrigin 设置origin
func (c *ChatClient) WithOrigin(origin string) *ChatClient {
	c.Origin = origin
	return c
}

// WithEngine 设置engine
func (c *ChatClient) WithEngine(engine string) *ChatClient {
	c.Engine = engine
	return c
}

// WithModel 设置模型
func (c *ChatClient) WithModel(model string) *ChatClient {
	c.Model = model
	return c
}

// WithHost 设置baseHost
func (c *ChatClient) WithHost(host string) *ChatClient {
	c.BaseHost = host
	return c
}

// WithHttpProxy 设置HTTP代理
func (c *ChatClient) WithHttpProxy(proxy string) *ChatClient {
	c.HttpProxy = proxy
	return c
}

// WithSocks5Proxy 设置SOCKS5代理
func (c *ChatClient) WithSocks5Proxy(proxy string) *ChatClient {
	c.Socks5Proxy = proxy
	return c
}

// WithProxyAuth 设置代理认证
func (c *ChatClient) WithProxyAuth(userName, password string) *ChatClient {
	c.ProxyUserName = userName
	c.ProxyPassword = password
	return c
}

// WithMaxToken 设置最大token数
func (c *ChatClient) WithMaxToken(maxToken int) *ChatClient {
	c.MaxToken = maxToken
	return c
}

// WithTotalToken 设置总token数
func (c *ChatClient) WithTotalToken(totalToken int) *ChatClient {
	c.TotalToken = totalToken
	return c
}

// WithTemperature 设置温度
func (c *ChatClient) WithTemperature(temperature float32) *ChatClient {
	c.Temperature = temperature
	return c
}

// WithDebug 设置debug
func (c *ChatClient) WithDebug(debug bool) *ChatClient {
	c.Debug = debug
	return c
}

// buildConfig 构建HTTP客户端配置
func (c *ChatClient) buildConfig() *http.Client {
	client := &http.Client{
		Timeout: 300 * time.Second,
	}

	// 设置代理
	if c.HttpProxy != "" {
		proxyUrl, _ := url.Parse(c.HttpProxy)
		if c.ProxyUserName != "" && c.ProxyPassword != "" {
			proxyUrl.User = url.UserPassword(c.ProxyUserName, c.ProxyPassword)
		}
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
	} else if c.Socks5Proxy != "" {
		socks5Transport := &http.Transport{}
		auth := proxy.Auth{}
		if c.ProxyUserName != "" && c.ProxyPassword != "" {
			auth.Password = c.ProxyPassword
			auth.User = c.ProxyUserName
		}
		dialer, _ := proxy.SOCKS5("tcp", c.Socks5Proxy, &auth, proxy.Direct)
		socks5Transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
		client.Transport = socks5Transport
	}

	return client
}

// buildHeaders 构建请求头
func (c *ChatClient) buildHeaders() map[string]string {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + c.APIKey,
	}

	if c.Origin != "" {
		headers["Origin"] = c.Origin
	}

	return headers
}
