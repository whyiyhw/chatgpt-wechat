package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"net/url"
)

type (
	EmbeddingModel string

	EmbeddingRequest struct {
		Model EmbeddingModel `json:"model"`
		Input string         `json:"input"`
	}

	EmbeddingResponse struct {
		Object string         `json:"object"`
		Data   []Embedding    `json:"data"`
		Model  EmbeddingModel `json:"model"`
		Usage  Usage          `json:"usage"`
	}

	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	}

	Embedding struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	}
)

type EmbeddingClient struct {
	APIKey         string         `json:"apiKey"`
	HttpProxy      string         `json:"http_proxy"`
	Socks5Proxy    string         `json:"socks5_proxy"`
	EmbeddingModel EmbeddingModel `json:"embeddingModel"`
}

const (
	ADA002                   EmbeddingModel = "text-embedding-ada-002"
	createEmbeddingsEndPoint string         = "https://api.openai.com/v1/embeddings"
)

func NewClient(apiKey string) EmbeddingClient {
	return EmbeddingClient{
		APIKey: apiKey,
	}
}

func (c *EmbeddingClient) WithModel(model EmbeddingModel) {
	c.EmbeddingModel = model
}

func (c *EmbeddingClient) WithHttpProxy(proxyUrl string) {
	c.HttpProxy = proxyUrl
}
func (c *EmbeddingClient) WithSocks5Proxy(proxyUrl string) {
	c.Socks5Proxy = proxyUrl
}

func (c *EmbeddingClient) CreateOpenAIEmbeddings(input string) (EmbeddingResponse, error) {
	requestBody := EmbeddingRequest{
		Model: c.EmbeddingModel,
		Input: input,
	}

	requestBodyBytes, _ := json.Marshal(requestBody)
	client := &http.Client{}

	//	设置 http 代理
	if c.HttpProxy != "" {
		httpTransport := &http.Transport{}
		dialer, _ := url.Parse(c.HttpProxy)
		httpTransport.Proxy = http.ProxyURL(dialer)
		client.Transport = httpTransport
	} else if c.Socks5Proxy != "" {
		//	设置 socks5 代理
		httpTransport := &http.Transport{}
		dialer, _ := proxy.SOCKS5("tcp", c.Socks5Proxy, nil, proxy.Direct)
		httpTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
		client.Transport = httpTransport
	}

	request, _ := http.NewRequest(http.MethodPost, createEmbeddingsEndPoint, bytes.NewReader(requestBodyBytes))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	response, err := client.Do(request)
	if err != nil {
		return EmbeddingResponse{}, err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)

	var openaiResponse EmbeddingResponse
	if err := json.NewDecoder(response.Body).Decode(&openaiResponse); err != nil {
		return EmbeddingResponse{}, err
	}

	return openaiResponse, nil
}
