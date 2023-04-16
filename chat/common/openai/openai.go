package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	copenai "github.com/sashabaranov/go-openai"
	"github.com/zeromicro/go-zero/core/logx"

	"golang.org/x/net/proxy"
)

const TextModel = "text-davinci-003"
const ChatModel = "gpt-3.5-turbo"
const ChatModelNew = "gpt-3.5-turbo-0301"
const ChatModel4 = "gpt-4"

const MaxToken = 2000
const Temperature = 0.8

type ChatModelMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatClient struct {
	APIKey      string  `json:"api_key"`
	HttpProxy   string  `json:"http_proxy"`
	Socks5Proxy string  `json:"socks5_proxy"`
	Model       string  `json:"model"`
	BaseHost    string  `json:"base_host"`
	MaxToken    int     `json:"max_token"`
	Temperature float32 `json:"temperature"`
}

func NewChatClient(apiKey string) *ChatClient {
	return &ChatClient{
		APIKey:      apiKey,
		MaxToken:    MaxToken,
		Temperature: Temperature,
	}
}

func (c *ChatClient) WithModel(model string) *ChatClient {
	if model != "" {
		c.Model = model
	}
	return c
}

func (c *ChatClient) WithBaseHost(baseHost string) *ChatClient {
	c.BaseHost = baseHost
	return c
}

// WithMaxToken 设置最大token数
func (c *ChatClient) WithMaxToken(maxToken int) *ChatClient {
	c.MaxToken = maxToken
	return c
}

// WithTemperature 设置响应灵活程度
func (c *ChatClient) WithTemperature(temperature float32) *ChatClient {
	c.Temperature = temperature
	return c
}

func (c *ChatClient) WithHttpProxy(proxyUrl string) *ChatClient {
	c.HttpProxy = proxyUrl
	return c
}
func (c *ChatClient) WithSocks5Proxy(proxyUrl string) *ChatClient {
	c.Socks5Proxy = proxyUrl
	return c
}

func (c *ChatClient) SpeakToTxt(voiceUrl string) (string, error) {
	config := c.buildConfig()
	cli := copenai.NewClientWithConfig(config)

	// 打印文件信息
	logx.Info("File: ", voiceUrl)
	info, err := os.Stat(voiceUrl)
	if err != nil {
		return "", err
	}

	logx.Info("FileInfo: ", info)

	req := copenai.AudioRequest{
		Model:       copenai.Whisper1,
		FilePath:    voiceUrl,
		Prompt:      "使用简体中文",
		Temperature: 0.5,
		Language:    "zh",
	}
	resp, err := cli.CreateTranscription(context.Background(), req)
	if err != nil {
		logx.Info("Transcription error: ", err)
		return "", err
	}

	// 用完就删掉
	_ = os.Remove(voiceUrl)

	return resp.Text, nil
}

func (c *ChatClient) Completion(req string) (string, error) {
	config := c.buildConfig()
	cli := copenai.NewClientWithConfig(config)

	// 打印请求信息
	logx.Info("Completion req: ", req)
	request := copenai.CompletionRequest{
		Model:       copenai.GPT3TextDavinci003,
		Prompt:      req,
		MaxTokens:   c.MaxToken,
		Temperature: c.Temperature,
		TopP:        1,
	}
	completion, err := cli.CreateCompletion(context.Background(), request)
	if err != nil {
		fmt.Println("req completion params:", config)
		return "", err
	}
	txt := ""
	for _, choice := range completion.Choices {
		txt += choice.Text
	}
	logx.Info("Completion reps: ", txt)
	return txt, nil
}

func (c *ChatClient) Chat(req []ChatModelMessage) (string, error) {

	config := c.buildConfig()
	cli := copenai.NewClientWithConfig(config)

	// 打印请求信息
	logx.Info("req: ", req)

	var messages []copenai.ChatCompletionMessage

	for _, message := range req {
		messages = append(messages, copenai.ChatCompletionMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	request := copenai.ChatCompletionRequest{
		Model:       ChatModel,
		Messages:    messages,
		MaxTokens:   c.MaxToken,
		Temperature: c.Temperature,
		TopP:        1,
	}
	chat, err := cli.CreateChatCompletion(context.Background(), request)
	if err != nil {
		fmt.Println("req chat params:", config)
		return "", err
	}
	txt := ""
	for _, choice := range chat.Choices {
		txt += choice.Message.Content
	}

	return txt, nil
}

func (c *ChatClient) buildConfig() copenai.ClientConfig {
	config := copenai.DefaultConfig(c.APIKey)
	if c.HttpProxy != "" {
		proxyUrl, _ := url.Parse(c.HttpProxy)
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
		config.HTTPClient = &http.Client{
			Transport: transport,
		}
	} else if c.Socks5Proxy != "" {
		socks5Transport := &http.Transport{}
		dialer, _ := proxy.SOCKS5("tcp", c.Socks5Proxy, nil, proxy.Direct)
		socks5Transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
		config.HTTPClient = &http.Client{
			Transport: socks5Transport,
		}
	}

	if c.BaseHost != "" {
		// trim last slash
		config.BaseURL = strings.TrimRight(c.BaseHost, "/") + "/v1"
	}
	return config
}

func (c *ChatClient) ChatStream(req []ChatModelMessage, channel chan string) (string, error) {

	config := c.buildConfig()

	cli := copenai.NewClientWithConfig(config)

	// 打印请求信息
	logx.Info("req: ", req)
	first := 0
	var system ChatModelMessage
	for i, msg := range req {
		if msg.Role == "system" {
			system = msg
		}
		if i%2 == 0 {
			continue
		}
		//估算长度
		if NumTokensFromMessages(req[len(req)-i-1:], ChatModel) < (3900 - c.MaxToken) {
			first = len(req) - i - 1
		} else {
			break
		}
	}

	var messages []copenai.ChatCompletionMessage

	if first != 0 {
		messages = append(messages, copenai.ChatCompletionMessage{
			Role:    system.Role,
			Content: system.Content,
		})
	}

	for _, message := range req[first:] {
		messages = append(messages, copenai.ChatCompletionMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	request := copenai.ChatCompletionRequest{
		Model:       ChatModel,
		Messages:    messages,
		MaxTokens:   c.MaxToken,
		Temperature: c.Temperature,
		TopP:        1,
	}
	stream, err := cli.CreateChatCompletionStream(context.Background(), request)

	if err != nil {
		fmt.Println("req chat stream params:", config)
		return "", err
	}
	defer stream.Close()

	messageText := ""
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			logx.Info("Stream finished")
			return messageText, nil
		}

		if err != nil {
			fmt.Printf("Stream error: %v\n", err)
			close(channel)
			return messageText, err
		}

		if len(response.Choices) > 0 {
			content := response.Choices[0].Delta.Content
			channel <- content
			// 如果是对内容的进行补充
			messageText += content
			// 结算
			if response.Choices[0].FinishReason != "" {
				close(channel)
				return messageText, nil
			}
		}

		logx.Info("Stream response:", response)
	}
}
