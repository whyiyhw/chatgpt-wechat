package dify

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Client struct {
	host             string
	defaultAPISecret string
	httpClient       *http.Client
}

func NewClientWithConfig(c *ClientConfig) *Client {
	var httpClient = &http.Client{}

	if c.Timeout != 0 {
		httpClient.Timeout = c.Timeout
	}
	if c.Transport != nil {
		httpClient.Transport = c.Transport
	}

	secret := c.DefaultAPISecret
	return &Client{
		host:             c.Host,
		defaultAPISecret: secret,
		httpClient:       httpClient,
	}
}

func NewClient(host, defaultAPISecret string) *Client {
	return NewClientWithConfig(&ClientConfig{
		Host:             host,
		DefaultAPISecret: defaultAPISecret,
	})
}

func (c *Client) sendRequest(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

func (c *Client) sendJSONRequest(req *http.Request, res interface{}) error {
	resp, err := c.sendRequest(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Status  int    `json:"status"`
		}
		if decErr := json.NewDecoder(resp.Body).Decode(&errBody); decErr != nil {
			// 错误体不是预期 JSON：至少保留 HTTP 状态码，便于上游分类。
			return &APIError{Status: resp.StatusCode, Message: decErr.Error()}
		}
		return &APIError{Status: resp.StatusCode, Code: errBody.Code, Message: errBody.Message}
	}

	err = json.NewDecoder(resp.Body).Decode(res)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) getHost() string {
	var host = strings.TrimSuffix(c.host, "/")
	return host
}

func (c *Client) getAPISecret() string {
	return c.defaultAPISecret
}

func (c *Client) API() *API {
	return &API{
		c: c,
	}
}
