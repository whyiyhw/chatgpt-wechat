package sensecore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

// SenseCoreCompletionRequest 商汤补全请求结构
type SenseCoreCompletionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float32 `json:"temperature,omitempty"`
	TopP        float32 `json:"top_p,omitempty"`
	N           int     `json:"n,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
	Thinking    struct {
		Enable bool
	} `json:"thinking"`
	Stop []string `json:"stop,omitempty"`
}

// SenseCoreCompletionResponse 商汤补全响应结构
type SenseCoreCompletionResponse struct {
	ID      string                      `json:"id"`
	Object  string                      `json:"object"`
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Choices []SenseCoreCompletionChoice `json:"choices"`
	Usage   SenseCoreUsage              `json:"usage"`
}

// SenseCoreCompletionChoice 商汤补全选择结构
type SenseCoreCompletionChoice struct {
	Text         string      `json:"text"`
	Index        int         `json:"index"`
	Logprobs     interface{} `json:"logprobs"`
	FinishReason string      `json:"finish_reason"`
}

// Completion 非流式文本补全
func (c *ChatClient) Completion(req []ChatModelMessage) (string, error) {
	_, request := c.commonCompletion(req)

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %v", err)
	}

	url := fmt.Sprintf("%s/v1/llm/completions", c.BaseHost)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request failed: %v", err)
	}

	// 设置请求头
	headers := c.buildHeaders()
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	client := c.buildConfig()
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response SenseCoreCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode response failed: %v", err)
	}

	txt := ""
	for _, choice := range response.Choices {
		txt += choice.Text
	}
	logx.Info("Completion reps: ", txt)
	return txt, nil
}

// CompletionStream 流式文本补全
func (c *ChatClient) CompletionStream(req []ChatModelMessage, channel chan string) (string, error) {
	_, request := c.commonCompletion(req)

	// 设置流式传输
	request.Stream = true
	request.Thinking.Enable = c.Debug

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %v", err)
	}

	url := fmt.Sprintf("%s/v1/llm/completions", c.BaseHost)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request failed: %v", err)
	}

	// 设置请求头
	headers := c.buildHeaders()
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	client := c.buildConfig()
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	messageText := ""
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				logx.Info("Stream finished")
				return messageText, nil
			}
			return messageText, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 处理 SSE 格式
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				close(channel)
				return messageText, nil
			}

			var streamResp SenseCoreCompletionResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				logx.Errorf("unmarshal stream response failed: %v", err)
				continue
			}

			if len(streamResp.Choices) > 0 {
				choice := streamResp.Choices[0]
				if choice.Text != "" {
					content := choice.Text
					channel <- content
					messageText += content
				}

				if choice.FinishReason != "" {
					close(channel)
					return messageText, nil
				}
			}
		}
	}
}

func (c *ChatClient) commonCompletion(req []ChatModelMessage) (*http.Client, *SenseCoreCompletionRequest) {
	client := c.buildConfig()

	// 打印请求信息
	logx.Info("req: ", req)
	first := 0
	var system ChatModelMessage
	for i, msg := range req {
		if msg.Role == SystemRole {
			system = msg
		}
		if i%2 == 0 {
			continue
		}
		//估算长度
		if NumTokensFromMessages(req[len(req)-i-1:], TextModel) < (c.TotalToken - c.MaxToken) {
			first = len(req) - i - 1
		} else {
			break
		}
	}

	var prompt string

	if first != 0 {
		prompt = system.Content.Data + "\n"
	}

	for _, message := range req[first:] {
		if message.Role == UserRole {
			prompt += "User: " + message.Content.Data + "\n"
		} else if message.Role == ModelRole {
			prompt += "Assistant: " + message.Content.Data + "\n"
		}
	}

	request := &SenseCoreCompletionRequest{
		Model:       c.Model,
		Prompt:      prompt,
		MaxTokens:   c.MaxToken,
		Temperature: c.Temperature,
	}

	return client, request
}
