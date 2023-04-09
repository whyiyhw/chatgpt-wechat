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

type TextModelRequest struct {
	Model            string   `json:"model"`
	Prompt           string   `json:"prompt"`
	MaxTokens        int      `json:"max_tokens"`
	Temperature      float64  `json:"temperature"`
	FrequencyPenalty int      `json:"frequency_penalty"`
	PresencePenalty  int      `json:"presence_penalty"`
	TopP             int      `json:"top_p"`
	Stream           bool     `json:"stream"`
	Stop             []string `json:"stop"`
}

type GoUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
type GoChoices struct {
	Text         string      `json:"text"`
	Index        int         `json:"index"`
	Logprobs     interface{} `json:"logprobs"`
	FinishReason string      `json:"finish_reason"`
}
type TextModelResult struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int         `json:"created"`
	Model   string      `json:"model"`
	Choices []GoChoices `json:"choices"`
	Usage   GoUsage     `json:"usage"`
}

type GoError struct {
	Message string      `json:"message"`
	Type    string      `json:"type"`
	Param   interface{} `json:"param"`
	Code    interface{} `json:"code"`
}

type ResultError struct {
	Error GoError `json:"error"`
}

type ChatModelRequest struct {
	Model            string             `json:"model"`
	Messages         []ChatModelMessage `json:"messages"`
	MaxTokens        int                `json:"max_tokens"`
	Temperature      float64            `json:"temperature"`
	FrequencyPenalty int                `json:"frequency_penalty"`
	PresencePenalty  int                `json:"presence_penalty"`
	TopP             int                `json:"top_p"`
	Stream           bool               `json:"stream"`
	Stop             []string           `json:"stop"`
}

type ChatModelMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatModelResult struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int                      `json:"created"`
	Choices []ChatModelResultChoices `json:"choices"`
	Usage   GoUsage                  `json:"usage"`
}

type ChatModelResultChoices struct {
	Index        int              `json:"index"`
	Message      ChatModelMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

type ChatClient struct {
	APIKey      string `json:"api_key"`
	HttpProxy   string `json:"http_proxy"`
	Socks5Proxy string `json:"socks5_proxy"`
	Model       string `json:"model"`
	BaseHost    string `json:"base_host"`
}

func NewChatClient(apiKey string) *ChatClient {
	return &ChatClient{
		APIKey: apiKey,
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
		MaxTokens:   2000,
		Temperature: 0.8,
		TopP:        1,
	}
	completion, err := cli.CreateCompletion(context.Background(), request)
	if err != nil {
		fmt.Println("req params:", config)
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
		MaxTokens:   2000,
		Temperature: 0.8,
		TopP:        1,
	}
	chat, err := cli.CreateChatCompletion(context.Background(), request)
	if err != nil {
		fmt.Println("req params:", config)
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
		MaxTokens:   2000,
		Temperature: 0.8,
		TopP:        1,
	}
	stream, err := cli.CreateChatCompletionStream(context.Background(), request)

	if err != nil {
		fmt.Println("req params:", config)
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
