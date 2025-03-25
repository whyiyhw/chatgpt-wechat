package deepseek

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

const ChatModel = "deepseek-chat"
const ReasonerModel = "deepseek-reasoner"

const UserRole = "user"
const SystemRole = "system"
const AssistantRole = "assistant"
const ToolRole = "tool"

const MimetypeTextPlain = "text/plain"

var ChatModelInputTokenLimit = map[string]int{
	ChatModel:     8192,
	ReasonerModel: 8192,
}

var ChatModelOutputTokenLimit = map[string]int{
	ChatModel:     4096,
	ReasonerModel: 4096,
}

var Temperature = 1.0
var DefaultHost = "https://api.deepseek.com"

type ChatModelMessage struct {
	MessageId string      `json:"message_id,omitempty"`
	Role      string      `json:"role"`
	Content   ChatContent `json:"content"`
}

func NewChatContent(data string) ChatContent {
	return ChatContent{
		MIMEType: MimetypeTextPlain,
		Data:     data,
	}
}

type ChatContent struct {
	MIMEType string `json:"mime_type,omitempty"`
	Data     string `json:"data"`
}

type ChatClient struct {
	HTTPClient    *http.Client `json:"-"`
	Host          string       `json:"host"`
	APIKey        string       `json:"api_key"`
	HttpProxy     string       `json:"http_proxy"`
	Socks5Proxy   string       `json:"socks5_proxy"`
	ProxyUserName string       `json:"proxy_user_name"`
	ProxyPassword string       `json:"proxy_password"`
	Model         string       `json:"model"`
	Temperature   float32      `json:"temperature"`
	Debug         bool         `json:"debug"`
}

func NewChatClient(apiKey string) *ChatClient {
	return &ChatClient{
		APIKey:      apiKey,
		Model:       ChatModel,
		Temperature: float32(Temperature),
		Host:        DefaultHost,
		Debug:       false,
	}
}

func (c *ChatClient) WithModel(model string) *ChatClient {
	c.Model = model
	return c
}

// WithHost 设置服务地址
func (c *ChatClient) WithHost(host string) *ChatClient {
	c.Host = host
	return c
}

// WithTemperature 设置响应灵活程度
func (c *ChatClient) WithTemperature(temperature float32) *ChatClient {
	c.Temperature = temperature
	return c
}

// WithHttpProxy 设置http代理
func (c *ChatClient) WithHttpProxy(proxyUrl string) *ChatClient {
	c.HttpProxy = proxyUrl
	return c
}

// WithSocks5Proxy 设置socks5代理
func (c *ChatClient) WithSocks5Proxy(proxyUrl string) *ChatClient {
	c.Socks5Proxy = proxyUrl
	return c
}

// WithProxyUserName 设置代理用户名
func (c *ChatClient) WithProxyUserName(userName string) *ChatClient {
	c.ProxyUserName = userName
	return c
}

// WithProxyPassword 设置代理密码
func (c *ChatClient) WithProxyPassword(password string) *ChatClient {
	c.ProxyPassword = password
	return c
}

// WithDebug 设置调试模式
func (c *ChatClient) WithDebug(debug bool) *ChatClient {
	c.Debug = debug
	return c
}

func (c *ChatClient) buildConfig() *ChatClient {
	if c.HttpProxy != "" {
		proxyUrl, _ := url.Parse(c.HttpProxy)
		if c.ProxyUserName != "" && c.ProxyPassword != "" {
			proxyUrl.User = url.UserPassword(c.ProxyUserName, c.ProxyPassword)
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
		c.HTTPClient = &http.Client{
			Transport: transport,
			Timeout:   300 * time.Second,
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
		c.HTTPClient = &http.Client{
			Transport: socks5Transport,
			Timeout:   300 * time.Second,
		}
	} else {
		c.HTTPClient = &http.Client{
			Timeout: 300 * time.Second,
		}
	}

	return c
}
