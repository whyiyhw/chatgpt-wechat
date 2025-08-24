package sensecore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

// SenseCoreChatRequest 商汤聊天请求结构
type SenseCoreChatRequest struct {
	Model       string             `json:"model"`
	Messages    []SenseCoreMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float32            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Thinking    struct {
		Enable bool
	} `json:"thinking"`
	Tools      []SenseCoreTool `json:"tools,omitempty"`
	ToolChoice interface{}     `json:"tool_choice,omitempty"`
}

// SenseCoreMessage 商汤消息结构
type SenseCoreMessage struct {
	Role      string              `json:"role"`
	Content   string              `json:"content"`
	Name      string              `json:"name,omitempty"`
	ToolCalls []SenseCoreToolCall `json:"tool_calls,omitempty"`
}

// SenseCoreTool 商汤工具结构
type SenseCoreTool struct {
	Type     string            `json:"type"`
	Function SenseCoreFunction `json:"function,omitempty"`
}

// SenseCoreFunction 商汤函数结构
type SenseCoreFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// SenseCoreToolCall 商汤工具调用结构
type SenseCoreToolCall struct {
	ID       string                `json:"id"`
	Type     string                `json:"type"`
	Function SenseCoreFunctionCall `json:"function"`
}

// SenseCoreFunctionCall 商汤函数调用结构
type SenseCoreFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// SenseCoreChatResponse 商汤聊天响应结构
type SenseCoreChatResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []SenseCoreChoice `json:"choices"`
	Usage   SenseCoreUsage    `json:"usage"`
}

// SenseCoreChoice 商汤选择结构
type SenseCoreChoice struct {
	Index        int               `json:"index"`
	Message      SenseCoreMessage  `json:"message"`
	FinishReason string            `json:"finish_reason"`
	Delta        *SenseCoreMessage `json:"delta,omitempty"`
}

// SenseCoreUsage 商汤使用统计结构
type SenseCoreUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatStream 数据流式传输
func (c *ChatClient) ChatStream(req []ChatModelMessage, channel chan string) (string, error) {
	_, request := c.commonChat(req)

	// 设置流式传输
	request.Stream = true

	// Thinking
	request.Thinking.Enable = c.Debug

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %v", err)
	}
	//[POST] https://api.sensenova.cn/v1/llm/chat-completions
	url := fmt.Sprintf("%s/v1/llm/chat-completions", c.BaseHost)

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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

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
				close(channel)
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

			var streamResp SenseCoreChatResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				logx.Errorf("unmarshal stream response failed: %v", err)
				continue
			}

			if len(streamResp.Choices) > 0 {
				choice := streamResp.Choices[0]
				if choice.Delta != nil && choice.Delta.Content != "" {
					content := choice.Delta.Content
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

// Chat 非流式聊天
func (c *ChatClient) Chat(req []ChatModelMessage) (string, error) {
	_, request := c.commonChat(req)

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %v", err)
	}

	url := fmt.Sprintf("%s/v1/llm/chat-completions", c.BaseHost)

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

	var response SenseCoreChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode response failed: %v", err)
	}

	if len(response.Choices) > 0 {
		return response.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response content")
}

func (c *ChatClient) commonChat(req []ChatModelMessage) (*http.Client, *SenseCoreChatRequest) {
	client := c.buildConfig()

	// 打印请求信息
	logx.Info("Chat stream req: ", req)
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
		if NumTokensFromMessages(req[len(req)-i-1:], ChatModel) < (c.TotalToken - c.MaxToken) {
			first = len(req) - i - 1
		} else {
			break
		}
	}

	var messages []SenseCoreMessage

	if first != 0 {
		messages = append(messages, SenseCoreMessage{
			Role:    SystemRole,
			Content: system.Content.Data,
		})
	}

	for _, message := range req[first:] {
		messages = append(messages, SenseCoreMessage{
			Role:    message.Role,
			Content: message.Content.Data,
		})
	}

	for i, message := range messages {
		if message.Role != SystemRole && message.Role != UserRole {
			messages[i].Role = ModelRole
		}
	}

	request := &SenseCoreChatRequest{
		Model:       c.Model,
		Messages:    messages,
		MaxTokens:   c.MaxToken,
		Temperature: c.Temperature,
	}

	return client, request
}

// NumTokensFromMessages 计算消息的token数量（简化实现）
func NumTokensFromMessages(messages []ChatModelMessage, model string) int {
	// 这里应该实现真实的token计算逻辑
	// 暂时返回一个估算值
	total := 0
	for _, msg := range messages {
		total += len(msg.Content.Data) / 4 // 粗略估算：1个token约等于4个字符
	}
	return total
}
