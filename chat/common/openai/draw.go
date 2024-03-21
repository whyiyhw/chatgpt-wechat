package openai

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	copenai "github.com/sashabaranov/go-openai"
	"github.com/zeromicro/go-zero/core/logx"

	"golang.org/x/net/proxy"
)

type Draw struct {
	Host          string `json:"host"`
	APIKey        string `json:"APIKey"`
	Origin        string `json:"origin"`
	Engine        string `json:"engine"`
	HttpProxy     string `json:"http_proxy"`
	Socks5Proxy   string `json:"socks5_proxy"`
	ProxyUserName string `json:"proxyUserName"`
	ProxyPassword string `json:"proxyPassword"`
}

func NewOpenaiDraw(host, key string) *Draw {
	return &Draw{
		Host:   host,
		APIKey: key,
	}
}

// WithOrigin 设置origin
func (od *Draw) WithOrigin(origin string) *Draw {
	od.Origin = origin
	return od
}

// WithEngine 设置engine
func (od *Draw) WithEngine(engine string) *Draw {
	od.Engine = engine
	return od
}

// WithHttpProxy 设置代理
func (od *Draw) WithHttpProxy(proxy string) *Draw {
	od.HttpProxy = proxy
	return od
}

// WithSocks5Proxy 设置socks5代理
func (od *Draw) WithSocks5Proxy(proxy string) *Draw {
	od.Socks5Proxy = proxy
	return od
}

// WithProxyUserName 设置代理用户名
func (od *Draw) WithProxyUserName(userName string) *Draw {
	od.ProxyUserName = userName
	return od
}

// WithProxyPassword 设置代理密码
func (od *Draw) WithProxyPassword(password string) *Draw {
	od.ProxyPassword = password
	return od
}

func (od *Draw) Txt2Img(prompt string, ch chan string) error {
	cli := copenai.NewClientWithConfig(od.buildConfig())

	ch <- "start"

	imgReq := copenai.ImageRequest{
		Prompt:         prompt,
		N:              1,
		Size:           copenai.CreateImageSize512x512,
		ResponseFormat: copenai.CreateImageResponseFormatB64JSON,
	}

	image, err := cli.CreateImage(context.Background(), imgReq)
	if err != nil {
		return err
	}

	// 读取
	if len(image.Data) > 0 {
		s := image.Data[0].B64JSON

		imageBase64 := strings.Split(s, ",")[0]
		decodeBytes, err := base64.StdEncoding.DecodeString(imageBase64)
		if err != nil {
			logx.Info("draw request fail", err)
			return errors.New("绘画请求响应Decode失败，请重新尝试~")
		}

		// 判断目录是否存在
		_, err = os.Stat("/tmp/image")
		if err != nil {
			err := os.MkdirAll("/tmp/image", os.ModePerm)
			if err != nil {
				fmt.Println("mkdir err:", err)
				return errors.New("绘画请求响应保存至目录失败，请重新尝试~")
			}
		}

		path := fmt.Sprintf("/tmp/image/%s.png", uuid.New().String())

		err = os.WriteFile(path, decodeBytes, os.ModePerm)

		if err != nil {
			logx.Info("draw save fail", err)
			return errors.New("绘画请求响应保存失败，请重新尝试~")
		}

		// 再将 image 信息发送到用户
		ch <- path
	}

	ch <- "stop"

	return nil
}

func (od *Draw) buildConfig() copenai.ClientConfig {
	config := copenai.DefaultConfig(od.APIKey)
	if od.Origin == "azure" || od.Origin == "azure_ad" {
		config = copenai.DefaultAzureConfig(od.APIKey, od.Host)
		// 默认只使用 一个部署的模型
		config.AzureModelMapperFunc = func(model string) string {
			return od.Engine
		}
	}

	if od.HttpProxy != "" {
		proxyUrl, _ := url.Parse(od.HttpProxy)
		if od.ProxyUserName != "" && od.ProxyPassword != "" {
			proxyUrl.User = url.UserPassword(od.ProxyUserName, od.ProxyPassword)
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
		config.HTTPClient = &http.Client{
			Transport: transport,
			Timeout:   300 * time.Second,
		}
	} else if od.Socks5Proxy != "" {
		socks5Transport := &http.Transport{}
		auth := proxy.Auth{}
		if od.ProxyUserName != "" && od.ProxyPassword != "" {
			auth.Password = od.ProxyPassword
			auth.User = od.ProxyUserName
		}
		dialer, _ := proxy.SOCKS5("tcp", od.Socks5Proxy, &auth, proxy.Direct)
		socks5Transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
		config.HTTPClient = &http.Client{
			Transport: socks5Transport,
			Timeout:   300 * time.Second,
		}
	}

	if od.Host != "" && od.Origin == "open_ai" {
		// trim last slash
		config.BaseURL = strings.TrimRight(od.Host, "/") + "/v1"
	}
	return config
}
