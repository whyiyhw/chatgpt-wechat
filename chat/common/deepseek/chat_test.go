package deepseek

import (
	"os"
	"testing"
	"time"
)

// 测试聊天功能
func TestChat(t *testing.T) {
	// 从环境变量获取API密钥，如果没有则跳过测试
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test because DEEPSEEK_API_KEY environment variable is not set")
	}

	client := NewChatClient(apiKey)
	client.WithDebug(true).WithModel(ReasonerModel)

	// 准备测试消息
	messages := []ChatModelMessage{
		{
			Role: SystemRole,
			Content: ChatContent{
				MIMEType: MimetypeTextPlain,
				Data:     "你是一个乐于助人的助手。",
			},
		},
		{
			Role: UserRole,
			Content: ChatContent{
				MIMEType: MimetypeTextPlain,
				Data:     "你好，请用一句话介绍一下自己。",
			},
		},
	}

	// 发送聊天请求
	response, err := client.Chat(messages)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// 检查响应是否为空
	if response == "" {
		t.Fatalf("Expected a response but got an empty string")
	}

	t.Logf("Response: %s", response)
}

// 测试流式聊天功能
func TestChatStream(t *testing.T) {
	//从环境变量获取API密钥，如果没有则跳过测试
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test because DEEPSEEK_API_KEY environment variable is not set")
	}

	client := NewChatClient(apiKey)
	client.WithDebug(true).WithModel(ReasonerModel).WithTemperature(1.5)

	// 准备测试消息
	messages := []ChatModelMessage{
		{
			Role: SystemRole,
			Content: ChatContent{
				MIMEType: MimetypeTextPlain,
				Data:     "你是一个乐于助人的助手。",
			},
		},
		{
			Role: UserRole,
			Content: ChatContent{
				MIMEType: MimetypeTextPlain,
				Data:     "请写一个简短的故事，100字左右。",
			},
		},
	}

	// 创建通道接收流式响应
	responseChan := make(chan string, 100)

	// 发送流式聊天请求
	err := client.ChatStream(messages, responseChan)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	// 接收并处理流式响应
	var streamResponse string
	timeout := time.After(30 * time.Second)
	done := false

	for !done {
		select {
		case chunk, ok := <-responseChan:
			if !ok {
				done = true
				break
			}
			streamResponse += chunk
			t.Logf("Received chunk: %s", chunk)
		case <-timeout:
			t.Fatalf("Test timed out after 30 seconds")
			done = true
		}
	}

	// 检查是否收到了响应
	if streamResponse == "" {
		t.Fatalf("Expected stream response but got empty string")
	}

	t.Logf("Full streamed response: %s", streamResponse)
}
