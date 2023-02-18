package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"chat/common/wecom"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/Masterminds/squirrel"
	"github.com/zeromicro/go-zero/core/logx"
)

type ChatLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatLogic {
	return &ChatLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChatLogic) Chat(req *types.ChatReq) (resp *types.ChatReply, err error) {

	if req.Channel == "openai" {
		go func() {
			// 去找 openai 获取数据
			url := "https://api.openai.com/v1/completions"
			method := "POST"

			type GoPayload struct {
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

			basePrompt := "你是 ChatGPT, 一个由 OpenAI 训练的大型语言模型, 你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。\n"

			if req.MSG == "#清除记忆" {
				// 去数据库删除 用户的所有对话数据
				builder := l.svcCtx.ChatModel.RowBuilder().Where(squirrel.Eq{"user": req.UserID}).OrderBy("id")
				collection, _ := l.svcCtx.ChatModel.FindAll(context.Background(), builder)
				for _, val := range collection {
					_ = l.svcCtx.ChatModel.Delete(context.Background(), val.Id)
				}
				wecom.SendToUser(req.AgentID, req.UserID, "记忆清除完成，来开始新一轮的chat吧", l.svcCtx.Config.WeCom.CorpID, l.svcCtx.Config.WeCom.CorpSecret)
				return
			}

			// 从数据库中读取 用户的所有 请求与 响应数据 进行请求拼接
			whereBuilder := l.svcCtx.ChatModel.RowBuilder().Where(squirrel.Eq{"user": req.UserID})
			collection, _ := l.svcCtx.ChatModel.FindAll(context.Background(), whereBuilder)
			skipNum := 0
			if len(collection) > 1000 {
				skipNum = len(collection) - 1000
			}
			for k, val := range collection {
				if k >= skipNum {
					basePrompt += "Q: " + val.ReqContent + "\nA: " + val.ResContent + "<|im_end|>\n"
				}
			}

			basePrompt += "\nQ: " + req.MSG + "\nA: "

			rq := GoPayload{
				"text-davinci-003",
				basePrompt,
				1200,
				0.9,
				0,
				0,
				1,
				false,
				[]string{"#"},
			}

			fmt.Println(basePrompt)

			bytes, _ := json.Marshal(rq)
			payload := strings.NewReader(string(bytes))

			client := &http.Client{}
			c, err := http.NewRequest(method, url, payload)

			if err != nil {
				fmt.Println(err)
				return
			}

			c.Header.Add("Authorization", "Bearer "+l.svcCtx.Config.OpenAi.Key)
			c.Header.Add("Content-Type", "application/json")

			res, err := client.Do(c)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {

				}
			}(res.Body)

			body, _ := io.ReadAll(res.Body)

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
			type OpenAiRes struct {
				ID      string      `json:"id"`
				Object  string      `json:"object"`
				Created int         `json:"created"`
				Model   string      `json:"model"`
				Choices []GoChoices `json:"choices"`
				Usage   GoUsage     `json:"usage"`
			}
			openAiRes := new(OpenAiRes)

			type GoError struct {
				Message string      `json:"message"`
				Type    string      `json:"type"`
				Param   interface{} `json:"param"`
				Code    interface{} `json:"code"`
			}
			type OpenAiResError struct {
				Error GoError `json:"error"`
			}
			openAiResError := new(OpenAiResError)

			fmt.Println(string(body))

			sysErr := json.Unmarshal(body, openAiResError)
			messageText := ""
			if sysErr != nil || openAiResError.Error.Type != "" {
				messageText = req.MSG + "\n\n系统过载请稍后再试"
			} else {
				_ = json.Unmarshal(body, openAiRes)

				if len(openAiRes.Choices) > 0 {
					messageText = strings.Replace(openAiRes.Choices[0].Text, "\n\n", "", 1)
					messageText = strings.Replace(messageText, "<|im_end|>", "", 1)
				} else {
					messageText = req.MSG + "\n\n未知错误,请稍后再试~"
				}
			}

			_, _ = l.svcCtx.ChatModel.Insert(context.Background(), &model.Chat{
				User:       req.UserID,
				ReqContent: req.MSG,
				ResContent: messageText,
			})

			// 然后把数据 发给微信用户
			wecom.SendToUser(req.AgentID, req.UserID, messageText, l.svcCtx.Config.WeCom.CorpID, l.svcCtx.Config.WeCom.CorpSecret)
		}()
	}

	if req.Channel == "wecom" {
		wecom.SendToUser(req.AgentID, req.UserID, req.MSG, l.svcCtx.Config.WeCom.CorpID, l.svcCtx.Config.WeCom.CorpSecret)
	}

	return &types.ChatReply{
		Message: "ok",
	}, nil
}
