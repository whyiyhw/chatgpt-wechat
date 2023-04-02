package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"chat/common/openai"
	"chat/common/wecom"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/Masterminds/squirrel"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/net/proxy"
)

type CustomerChatLogic struct {
	logx.Logger
	ctx        context.Context
	svcCtx     *svc.ServiceContext
	model      string
	baseHost   string
	basePrompt string
	message    string
}

func NewCustomerChatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerChatLogic {
	return &CustomerChatLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CustomerChatLogic) CustomerChat(req *types.CustomerChatReq) (resp *types.CustomerChatReply, err error) {

	url := "https://api.openai.com/v1/completions"

	l.setModelName().setBasePrompt().setBaseHost()

	// 确认消息没有被处理过

	_, err = l.svcCtx.ChatModel.FindOneByQuery(context.Background(),
		l.svcCtx.ChatModel.RowBuilder().Where(squirrel.Eq{"message_id": req.MsgID}).Where(squirrel.Eq{"user": req.CustomerID}),
	)
	// 消息已处理
	if err == nil {
		return &types.CustomerChatReply{
			Message: "ok",
		}, nil
	}
	// 然后 把 消息 发给 openai
	go func() {
		// 从数据库中读取 用户的所有 请求与 响应数据 进行请求拼接
		whereBuilder := l.svcCtx.ChatModel.RowBuilder().Where(squirrel.Eq{"user": req.CustomerID}).
			Where(squirrel.Eq{"open_kf_id": req.OpenKfID}).
			Limit(1).
			OrderBy("`id` desc")
		collection, _ := l.svcCtx.ChatModel.FindAll(context.Background(), whereBuilder)

		var bytes []byte

		var prompts []openai.ChatModelMessage
		prompts = append(prompts, openai.ChatModelMessage{
			Role:    "system",
			Content: l.basePrompt,
		})

		for _, val := range collection {
			prompts = append(prompts, openai.ChatModelMessage{
				Role:    "user",
				Content: val.ReqContent,
			})
			prompts = append(prompts, openai.ChatModelMessage{
				Role:    "assistant",
				Content: val.ResContent,
			})
		}

		prompts = append(prompts, openai.ChatModelMessage{
			Role:    "user",
			Content: req.Msg,
		})

		bytes = openai.ChatRequestBuild(prompts)
		url = l.baseHost + "/v1/chat/completions"

		payload := strings.NewReader(string(bytes))

		client := &http.Client{}

		// 是否开启代理
		if l.svcCtx.Config.Proxy.Enable {
			dialer, err := proxy.SOCKS5("tcp", l.svcCtx.Config.Proxy.Socket5, nil, proxy.Direct)
			if err != nil {
				//sendToUser(req.AgentID, req.UserID, "代理设置失败"+err.Error(), l.svcCtx.Config)
				return
			}
			//	设置传输方式
			httpTransport := &http.Transport{}
			//	设置 socks5 代理
			httpTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}

			client.Transport = httpTransport
		}

		c, err := http.NewRequest(http.MethodPost, url, payload)

		if err != nil {
			fmt.Println("openai client request build fail:" + err.Error())
			return
		}

		c.Header.Add("Authorization", "Bearer "+l.svcCtx.Config.OpenAi.Key)
		c.Header.Add("Content-Type", "application/json")

		res, err := client.Do(c)
		if err != nil {
			fmt.Println("openai client req ing fail:" + err.Error())
			return
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {

			}
		}(res.Body)

		body, _ := io.ReadAll(res.Body)

		// 成功

		openAiResError := new(openai.ResultError)

		fmt.Println("openai response: " + string(body))

		sysErr := json.Unmarshal(body, openAiResError)
		messageText := ""
		if sysErr != nil || openAiResError.Error.Type != "" {
			// 请求错误
			if openAiResError.Error.Type == "invalid_request_error" {
				switch openAiResError.Error.Code.(type) {
				case string:
					if openAiResError.Error.Code.(string) == "context_length_exceeded" {
						wecom.SendCustomerChatMessage(req.OpenKfID, req.CustomerID, "上下文超过最大长度的限制，请输入 #clear 来清理上下文")
						return
					}
				}
			}

			messageText = fmt.Sprintf("%s \n\n系统错误，请清理后重试: type:%v code:%v",
				req.Msg, openAiResError.Error.Type, openAiResError.Error.Code,
			)
		}

		if messageText == "" {
			if l.model == openai.TextModel {
				messageText = openai.GetTextModelResult(body)
			} else {
				messageText = openai.GetChatModelResult(body)
			}
		}

		_, _ = l.svcCtx.ChatModel.Insert(context.Background(), &model.Chat{
			User:       req.CustomerID,
			OpenKfId:   req.OpenKfID,
			MessageId:  req.MsgID,
			ReqContent: req.Msg,
			ResContent: messageText,
		})

		// 然后把数据 发给对应的客户
		wecom.SendCustomerChatMessage(req.OpenKfID, req.CustomerID, messageText)
	}()

	return &types.CustomerChatReply{
		Message: "ok",
	}, nil
}

func (l *CustomerChatLogic) setModelName() (ls *CustomerChatLogic) {
	m := l.svcCtx.Config.WeCom.Model
	if m == "" || (m != openai.TextModel && m != openai.ChatModel && m != openai.ChatModelNew && m != openai.ChatModel4) {
		m = openai.TextModel
	}
	l.svcCtx.Config.WeCom.Model = m
	return l
}

func (l *CustomerChatLogic) setBasePrompt() (ls *CustomerChatLogic) {
	p := l.svcCtx.Config.WeCom.BasePrompt
	if p == "" {
		p = "你是 ChatGPT, 一个由 OpenAI 训练的大型语言模型, 你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。\n"
	}
	l.basePrompt = p
	return l
}

func (l *CustomerChatLogic) setBaseHost() (ls *CustomerChatLogic) {
	if l.svcCtx.Config.OpenAi.Host == "" {
		l.svcCtx.Config.OpenAi.Host = "https://api.openai.com"
	}
	l.baseHost = l.svcCtx.Config.OpenAi.Host
	return l
}
