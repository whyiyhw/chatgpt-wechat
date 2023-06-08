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

	"github.com/google/uuid"
	"github.com/pkg/errors"
	copenai "github.com/sashabaranov/go-openai"
	"github.com/zeromicro/go-zero/core/logx"

	"golang.org/x/net/proxy"
)

type Draw struct {
	Host   string
	APIKey string
	Proxy  string
}

func NewOpenaiDraw(key, proxy string) *Draw {
	return &Draw{
		Host:   "https://api.openai.com",
		APIKey: key,
		Proxy:  proxy,
	}
}

// WithProxy 设置代理
func (od *Draw) WithProxy(proxy string) *Draw {
	od.Proxy = proxy
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
	if od.Proxy != "" {
		if strings.HasPrefix(od.Proxy, "http") {
			proxyUrl, err := url.Parse(od.Proxy)
			if err != nil {
				logx.Error("proxy parse error", err)
			}
			transport := &http.Transport{
				Proxy: http.ProxyURL(proxyUrl),
			}
			config.HTTPClient = &http.Client{
				Transport: transport,
			}
		} else {
			socks5Transport := &http.Transport{}
			dialer, _ := proxy.SOCKS5("tcp", od.Proxy, nil, proxy.Direct)
			socks5Transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
			config.HTTPClient = &http.Client{
				Transport: socks5Transport,
			}
		}
	}
	// trim last slash
	config.BaseURL = strings.TrimRight(od.Host, "/") + "/v1"

	return config
}
