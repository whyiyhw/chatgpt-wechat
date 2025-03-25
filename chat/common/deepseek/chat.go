package deepseek

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

type ResponseMessage struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	Role             string `json:"role"`
}

type Choice struct {
	Message      ResponseMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Index        int             `json:"index"`
}

type DeltaMessage struct {
	Content          string `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	Role             string `json:"role,omitempty"`
}

type StreamChoice struct {
	Delta        DeltaMessage `json:"delta"`
	FinishReason string       `json:"finish_reason"`
	Index        int          `json:"index"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Response struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type StreamResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// commonChat 处理消息验证和上下文管理
func (c *ChatClient) commonChat(chatRequest []ChatModelMessage) ([]ChatModelMessage, error) {
	// 1. 验证消息顺序
	var validMessages []ChatModelMessage
	hasSystem := false
	lastRole := ""

	for i, msg := range chatRequest {
		// 检查第一条消息是否为 system
		if i == 0 && msg.Role == SystemRole {
			hasSystem = true
			validMessages = append(validMessages, msg)
			lastRole = SystemRole
			continue
		}

		// 如果不是第一条消息，检查顺序
		if msg.Role == UserRole && (lastRole == SystemRole || lastRole == AssistantRole || lastRole == "") {
			validMessages = append(validMessages, msg)
			lastRole = UserRole
		} else if msg.Role == AssistantRole && lastRole == UserRole {
			validMessages = append(validMessages, msg)
			lastRole = AssistantRole
		} else {
			// 如果顺序不对，跳过这条消息
			logx.Info("Skip invalid message sequence, role: ", msg.Role, " last role: ", lastRole)
			continue
		}
	}

	// 如果最后一条消息不是 user，移除最后一组对话
	if len(validMessages) > 0 && validMessages[len(validMessages)-1].Role != UserRole {
		if len(validMessages) >= 2 {
			validMessages = validMessages[:len(validMessages)-2]
		} else {
			validMessages = []ChatModelMessage{}
		}
	}

	// 2. 处理上下文长度
	var finalMessages []ChatModelMessage
	var systemMsg ChatModelMessage

	// 保存 system 消息
	if hasSystem {
		systemMsg = validMessages[0]
		validMessages = validMessages[1:]
	}

	// 从后向前计算 token，确保不超过限制
	currentTokens := 0
	if hasSystem {
		currentTokens += NumTokensFromMessages([]ChatModelMessage{systemMsg}, c.Model)
	}

	// 从最新的消息开始添加
	for i := len(validMessages) - 1; i >= 0; i-- {
		tokensForMessage := NumTokensFromMessages([]ChatModelMessage{validMessages[i]}, c.Model)
		if currentTokens+tokensForMessage < ChatModelInputTokenLimit[c.Model] {
			finalMessages = append([]ChatModelMessage{validMessages[i]}, finalMessages...)
			currentTokens += tokensForMessage
		} else {
			break
		}
	}

	// 添加 system 消息到开头
	if hasSystem {
		finalMessages = append([]ChatModelMessage{systemMsg}, finalMessages...)
	}

	return finalMessages, nil
}

// Chat 实现聊天功能
func (c *ChatClient) Chat(chatRequest []ChatModelMessage) (txt string, err error) {
	// 使用 commonChat 处理消息
	validMessages, err := c.commonChat(chatRequest)
	if err != nil {
		return "", err
	}

	client := c.buildConfig().HTTPClient

	// 构建请求体
	messages := make([]map[string]interface{}, 0, len(validMessages))
	for _, msg := range validMessages {
		role := msg.Role
		// 确保角色符合 deepseek API 要求
		if role == "model" {
			role = "assistant"
		}
		messageMap := map[string]interface{}{
			"role":    role,
			"content": msg.Content.Data,
		}
		messages = append(messages, messageMap)
	}

	reqBody := map[string]interface{}{
		"model":       c.Model,
		"messages":    messages,
		"temperature": c.Temperature,
		"stream":      false,
	}

	// 为 ReasonerModel 添加特定参数
	if c.Model == ReasonerModel {
		reqBody["max_tokens"] = 4096
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return "", errors.Wrap(err, "marshal request body")
	}

	if c.Debug {
		fmt.Println("request body: " + string(reqData))
	}

	// 构建HTTP请求
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/chat/completions", c.Host),
		bytes.NewReader(reqData),
	)
	if err != nil {
		return "", errors.Wrap(err, "create request")
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.APIKey)

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "send request")
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logx.Error("close response body error: " + err.Error())
		}
	}(resp.Body)

	// 读取并解析响应
	bodyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "read response body")
	}

	if c.Debug {
		fmt.Println("response status: " + resp.Status)
		fmt.Println("response body: " + string(bodyData))
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s, status code: %d", string(bodyData), resp.StatusCode)
	}

	var response Response
	err = json.Unmarshal(bodyData, &response)
	if err != nil {
		return "", errors.Wrap(err, "unmarshal response")
	}

	if len(response.Choices) > 0 {
		txt = ""
		if response.Choices[0].Message.ReasoningContent != "" {
			txt += response.Choices[0].Message.ReasoningContent
			txt += "\n\n"
		}
		txt += response.Choices[0].Message.Content
	}

	return txt, nil
}

// ChatStream 实现流式聊天功能
func (c *ChatClient) ChatStream(chatRequest []ChatModelMessage, channel chan string) error {
	// 使用 commonChat 处理消息
	validMessages, err := c.commonChat(chatRequest)
	if err != nil {
		close(channel)
		return err
	}

	client := c.buildConfig().HTTPClient

	// 构建请求体
	messages := make([]map[string]interface{}, 0, len(validMessages))
	for _, msg := range validMessages {
		role := msg.Role
		// 确保角色符合 deepseek API 要求
		if role == "model" {
			role = "assistant"
		}
		messageMap := map[string]interface{}{
			"role":    role,
			"content": msg.Content.Data,
		}
		messages = append(messages, messageMap)
	}

	reqBody := map[string]interface{}{
		"model":       c.Model,
		"messages":    messages,
		"temperature": c.Temperature,
		"stream":      true,
	}

	// 为 ReasonerModel 添加特定参数
	if c.Model == ReasonerModel {
		reqBody["max_tokens"] = 4096
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		close(channel)
		return errors.Wrap(err, "marshal request body")
	}

	if c.Debug {
		fmt.Println("request body: " + string(reqData))
	}

	// 构建HTTP请求
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/chat/completions", c.Host),
		bytes.NewReader(reqData),
	)
	if err != nil {
		close(channel)
		return errors.Wrap(err, "create request")
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.APIKey)
	req.Header.Add("Accept", "text/event-stream")
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Add("Connection", "keep-alive")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		close(channel)
		return errors.Wrap(err, "send request")
	}

	// 处理SSE流
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
			return i + 2, data[0:i], nil
		}
		if i := bytes.Index(data, []byte("\n")); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	go func() {
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				logx.Error("close response body error: " + err.Error())
			}
		}(resp.Body)

		var fullResponse string
		reasonEnd := false

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			if data == "[DONE]" {
				break
			}

			var streamResp StreamResponse

			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				logx.Error("unmarshal stream response error: " + err.Error())
				continue
			}

			if len(streamResp.Choices) > 0 {
				// 处理常规文本内容
				content := streamResp.Choices[0].Delta.Content
				if content != "" {
					if c.Model == ReasonerModel && !reasonEnd {
						content = "\n\n" + content
						reasonEnd = true
					}
					channel <- content
					fullResponse += content
				}

				// 处理推理模型的reasoning_content
				if c.Model == ReasonerModel {
					reasoningContent := streamResp.Choices[0].Delta.ReasoningContent
					if reasoningContent != "" {
						if c.Debug {
							fmt.Println("Reasoning content: " + reasoningContent)
						}
						channel <- reasoningContent
						fullResponse += reasoningContent
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			logx.Error("scanner error: " + err.Error())
		}

		if c.Debug {
			fmt.Println("Full response: " + fullResponse)
		}

		close(channel)
	}()

	return nil
}
