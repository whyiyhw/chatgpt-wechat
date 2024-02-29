package openai

import (
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"
	copenai "github.com/sashabaranov/go-openai"
	"github.com/zeromicro/go-zero/core/logx"
)

func (c *ChatClient) Completion(req []ChatModelMessage) (string, error) {
	config, cli, request := c.commonCompletion(req)

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

func (c *ChatClient) CompletionStream(req []ChatModelMessage, channel chan string) (string, error) {

	config, cli, request := c.commonCompletion(req)

	stream, err := cli.CreateCompletionStream(context.Background(), request)

	if err != nil {
		fmt.Println("req completion stream params:", config)
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
			content := response.Choices[0].Text
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

func (c *ChatClient) commonCompletion(req []ChatModelMessage) (copenai.ClientConfig, *copenai.Client, copenai.CompletionRequest) {
	config := c.buildConfig()

	cli := copenai.NewClientWithConfig(config)

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

	var messages []copenai.ChatCompletionMessage

	if first != 0 {
		messages = append(messages, copenai.ChatCompletionMessage{
			Role:    system.Role,
			Content: system.Content.Data,
		})
	}

	for _, message := range req[first:] {
		messages = append(messages, copenai.ChatCompletionMessage{
			Role:    message.Role,
			Content: message.Content.Data,
		})
	}
	if _, ok := Models[c.Model]; !ok {
		c.Model = TextModel
	}
	for i, message := range messages {
		if message.Role != SystemRole && message.Role != UserRole {
			messages[i].Role = ModelRole
		}
	}
	lastPrompt := ""
	request := copenai.CompletionRequest{
		Model:       c.Model,
		MaxTokens:   c.MaxToken,
		Temperature: c.Temperature,
		TopP:        1,
	}
	if first != 0 {
		lastPrompt = system.Content.Data + "\n"
	}
	l := len(req[first:])
	for k, val := range req[first:] {
		switch val.Role {
		case UserRole:
			lastPrompt += "Q: " + val.Content.Data + "\n"
			if l == k+1 {
				lastPrompt += "A: "
			}
		case ModelRole:
			lastPrompt += "A: " + val.Content.Data + "\n"
		}
	}
	request.Prompt = lastPrompt
	return config, cli, request
}
