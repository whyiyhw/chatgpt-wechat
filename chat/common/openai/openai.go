package openai

import (
	"encoding/json"
	"fmt"
	"strings"
)

const TextModel = "text-davinci-003"
const ChatModel = "gpt-3.5-turbo"
const ChatModelNew = "gpt-3.5-turbo-0301"

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

func TextModelRequestBuild(basePrompt string) []byte {

	rq := TextModelRequest{
		Model:       TextModel,
		Prompt:      basePrompt,
		MaxTokens:   1200,
		Temperature: 0.8,
		TopP:        1,
		Stop:        []string{"#"},
	}

	bytes, _ := json.Marshal(rq)
	fmt.Println("request :" + string(bytes))
	return bytes
}
func GetTextModelResult(body []byte) (messageText string) {
	openAiRes := new(TextModelResult)
	_ = json.Unmarshal(body, openAiRes)

	if len(openAiRes.Choices) > 0 {
		messageText = strings.Replace(openAiRes.Choices[0].Text, "\n\n", "", 1)
		messageText = strings.Replace(messageText, "<|im_end|>", "", 1)
	} else {
		messageText = "未知错误,请稍后再试~"
	}
	return
}

func ChatRequestBuild(prompts []ChatModelMessage) []byte {
	rq := ChatModelRequest{
		Model:       ChatModel,
		Messages:    prompts,
		MaxTokens:   1200,
		Temperature: 0.8,
		TopP:        1,
		Stop:        []string{"#"},
	}

	bytes, _ := json.Marshal(rq)

	fmt.Println("request :" + string(bytes))
	return bytes
}

func GetChatModelResult(body []byte) (messageText string) {
	openAiRes := new(ChatModelResult)
	_ = json.Unmarshal(body, openAiRes)

	if len(openAiRes.Choices) > 0 {
		messageText = strings.Replace(openAiRes.Choices[0].Message.Content, "\n\n", "", 1)
	} else {
		messageText = "未知错误,请稍后再试~"
	}
	if messageText == "" {
		messageText = "啊咧咧, ai 给出了一个空白的响应，也许你应该换一个问法？"
	}
	return
}
