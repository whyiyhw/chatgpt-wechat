package logic

import (
	"chat/common/openai"
	"chat/service/chat/api/internal/config"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/net/proxy"
	"io"
	"net"
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

			baseHost := getBaseHost(l.svcCtx.Config)
			basePrompt := getBasePrompt(req.AgentID, l.svcCtx.Config)
			m := getModelName(req.AgentID, l.svcCtx.Config)

			// 如果用户有自定义的配置，就使用用户的配置
			configBuilder := l.svcCtx.ChatConfigModel.RowBuilder().
				Where(squirrel.Eq{"user": req.UserID}).
				Where(squirrel.Eq{"agent_id": req.AgentID}).
				OrderBy("id")
			configCollection, configErr := l.svcCtx.ChatConfigModel.FindOneByQuery(context.Background(), configBuilder)
			if configErr == nil {
				if configCollection.Id > 0 {
					basePrompt = configCollection.Prompt
					m = configCollection.Model
				}
			}

			//当 message 以 # 开头时，表示是特殊指令
			if strings.Contains(req.MSG, "#clear") {
				// 去数据库删除 用户的所有对话数据
				builder := l.svcCtx.ChatModel.RowBuilder().
					Where(squirrel.Eq{"user": req.UserID}).
					Where(squirrel.Eq{"agent_id": req.AgentID}).
					OrderBy("id")
				collection, _ := l.svcCtx.ChatModel.FindAll(context.Background(), builder)
				for _, val := range collection {
					_ = l.svcCtx.ChatModel.Delete(context.Background(), val.Id)
				}
				sendToUser(req.AgentID, req.UserID, "记忆清除完成，来开始新一轮的chat吧", l.svcCtx.Config)
				return
			} else if strings.Contains(req.MSG, "#help") {
				tips := fmt.Sprintf(
					"目前支持的指令有：\n %s\n %s\n %s\n %s\n %s\n %s",
					"#clear 清空当前应用的对话数据",
					"#system 查看当前对话的系统信息",
					"#config_prompt:您的设置 如 程序员的小助手",
					"#config_model:您的设置 如 text-davinci-003",
					"#config_clear:恢复当前应用的对话设置至初始值",
					"#help 查看所有指令",
				)
				sendToUser(req.AgentID, req.UserID, tips, l.svcCtx.Config)
				return
			} else if strings.Contains(req.MSG, "#system") {
				tips := "系统信息\n model 版本为：" + m + "\n 系统基础设定：" + basePrompt + " \n"
				sendToUser(req.AgentID, req.UserID, tips, l.svcCtx.Config)
				return
			} else if strings.Contains(req.MSG, "#config_prompt:") {
				// #config_prompt:您的设置 如 程序员的小助手\n
				// 处理 msg
				msg := strings.Replace(req.MSG, "#config_prompt:", "", -1)
				if msg == "" {
					sendToUser(req.AgentID, req.UserID, "请输入完整的设置 如：#config_prompt:程序员的小助手", l.svcCtx.Config)
					return
				}
				// 去数据库新增用户的对话配置
				chatConfig := model.ChatConfig{
					AgentId: req.AgentID,
					User:    req.UserID,
					Prompt:  msg,
					Model:   m,
				}
				_, configErr := l.svcCtx.ChatConfigModel.Insert(context.Background(), &chatConfig)

				if configErr != nil {
					sendToUser(req.AgentID, req.UserID, "设置失败,请稍后再试~", l.svcCtx.Config)
					return
				}

				sendToUser(req.AgentID, req.UserID, "设置成功，您目前的对话配置如下：\n prompt: "+msg+"\n model: "+m, l.svcCtx.Config)
				return
			} else if strings.Contains(req.MSG, "#config_model:") {
				// #config_model:您的设置 如 text-davinci-003\n
				msg := strings.Trim(strings.Replace(req.MSG, "#config_model:", "", -1), " ")

				if msg == "" {
					sendToUser(req.AgentID, req.UserID, "请输入完整的设置 如：\n#config_model:text-davinci-003", l.svcCtx.Config)
					return
				}

				if msg != openai.TextModel && msg != openai.ChatModel && msg != openai.ChatModelNew {
					tips := fmt.Sprintf("目前只支持以下三种模型：\n %s \n %s \n %s \n", openai.TextModel, openai.ChatModel, openai.ChatModelNew)
					sendToUser(req.AgentID, req.UserID, tips, l.svcCtx.Config)
					return
				}

				// 去数据库新增用户的对话配置
				chatConfig := model.ChatConfig{
					AgentId: req.AgentID,
					User:    req.UserID,
					Prompt:  basePrompt,
					Model:   msg,
				}
				_, configErr := l.svcCtx.ChatConfigModel.Insert(context.Background(), &chatConfig)

				if configErr != nil {
					sendToUser(req.AgentID, req.UserID, "设置失败,请稍后再试~", l.svcCtx.Config)
					return
				}

				sendToUser(req.AgentID, req.UserID, "设置成功，您目前的对话配置如下：\n prompt: "+basePrompt+"\n model: "+msg, l.svcCtx.Config)
				return

			} else if strings.Contains(req.MSG, "#config_clear") {
				// 去数据库删除 用户的所有对话配置
				builder := l.svcCtx.ChatConfigModel.RowBuilder().Where(squirrel.Eq{"user": req.UserID}).Where(squirrel.Eq{"agent_id": req.AgentID})
				collection, _ := l.svcCtx.ChatConfigModel.FindAll(context.Background(), builder)
				for _, val := range collection {
					_ = l.svcCtx.ChatConfigModel.Delete(context.Background(), val.Id)
				}
				sendToUser(req.AgentID, req.UserID, "对话配置清除完成", l.svcCtx.Config)
				return
			}

			// 从数据库中读取 用户的所有 请求与 响应数据 进行请求拼接
			whereBuilder := l.svcCtx.ChatModel.RowBuilder().Where(squirrel.Eq{"user": req.UserID}).Where(squirrel.Eq{"agent_id": req.AgentID})
			collection, _ := l.svcCtx.ChatModel.FindAll(context.Background(), whereBuilder)

			var bytes []byte
			skipNum := 0
			if len(collection) > 20 {
				skipNum = len(collection) - 20
			}
			if m == openai.TextModel {
				// TODO 将对话进行总结 然后拿总结的话进行构造请求与回复
				for k, val := range collection {
					if k >= skipNum {
						basePrompt += "Q: " + val.ReqContent + "\nA: " + val.ResContent + "<|im_end|>\n"
					}
				}
				basePrompt += "\nQ: " + req.MSG + "\nA: "

				bytes = openai.TextModelRequestBuild(basePrompt)

				url = baseHost + "/v1/completions"
			} else {
				var prompts []openai.ChatModelMessage
				prompts = append(prompts, openai.ChatModelMessage{
					Role:    "system",
					Content: basePrompt,
				})

				for k, val := range collection {
					if k >= skipNum {
						prompts = append(prompts, openai.ChatModelMessage{
							Role:    "user",
							Content: val.ReqContent,
						})
						prompts = append(prompts, openai.ChatModelMessage{
							Role:    "assistant",
							Content: val.ResContent,
						})
					}
				}

				prompts = append(prompts, openai.ChatModelMessage{
					Role:    "user",
					Content: req.MSG,
				})

				bytes = openai.ChatRequestBuild(prompts)
				url = baseHost + "/v1/chat/completions"
			}

			payload := strings.NewReader(string(bytes))

			client := &http.Client{}

			// 是否开启代理
			if l.svcCtx.Config.Proxy.Enable {
				dialer, err := proxy.SOCKS5("tcp", l.svcCtx.Config.Proxy.Socket5, nil, proxy.Direct)
				if err != nil {
					sendToUser(req.AgentID, req.UserID, "代理设置失败"+err.Error(), l.svcCtx.Config)
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

			c, err := http.NewRequest(method, url, payload)

			if err != nil {
				fmt.Println("client request build fail:" + err.Error())
				return
			}

			c.Header.Add("Authorization", "Bearer "+l.svcCtx.Config.OpenAi.Key)
			c.Header.Add("Content-Type", "application/json")

			res, err := client.Do(c)
			if err != nil {
				fmt.Println("client req ing fail:" + err.Error())
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

			fmt.Println("response: " + string(body))

			sysErr := json.Unmarshal(body, openAiResError)
			messageText := ""
			if sysErr != nil || openAiResError.Error.Type != "" {

				messageText = fmt.Sprintf("%s \n\n系统错误，请清理后重试: type:%v code:%v",
					req.MSG, openAiResError.Error.Type, openAiResError.Error.Code,
				)
			}

			if messageText == "" {
				if m == openai.TextModel {
					messageText = openai.GetTextModelResult(body)
				} else {
					messageText = openai.GetChatModelResult(body)
				}
			}

			_, _ = l.svcCtx.ChatModel.Insert(context.Background(), &model.Chat{
				AgentId:    req.AgentID,
				User:       req.UserID,
				ReqContent: req.MSG,
				ResContent: messageText,
			})

			// 然后把数据 发给微信用户
			sendToUser(req.AgentID, req.UserID, messageText, l.svcCtx.Config)
		}()
	}

	if req.Channel == "wecom" {
		sendToUser(req.AgentID, req.UserID, req.MSG, l.svcCtx.Config)
	}

	return &types.ChatReply{
		Message: "ok",
	}, nil
}

func getBaseHost(c config.Config) string {
	if c.OpenAi.Host != "" {
		return c.OpenAi.Host
	}
	return "https://api.openai.com"
}

func sendToUser(agentID int64, userID, msg string, config config.Config) {
	// 先去查询多应用模式是否开启
	corpSecret := config.WeCom.CorpSecret
	for _, application := range config.WeCom.MultipleApplication {
		if application.AgentID == agentID {
			corpSecret = application.AgentSecret
		}
	}
	wecom.SendToUser(agentID, userID, msg, config.WeCom.CorpID, corpSecret)
}

func getModelName(agentID int64, config config.Config) string {
	m := config.WeCom.Model
	for _, application := range config.WeCom.MultipleApplication {
		if application.AgentID == agentID {
			m = application.Model
		}
	}

	if m == "" || (m != openai.TextModel && m != openai.ChatModel && m != openai.ChatModelNew) {
		m = openai.TextModel
	}
	return m
}

func getBasePrompt(agentID int64, config config.Config) string {
	p := config.WeCom.BasePrompt
	for _, application := range config.WeCom.MultipleApplication {
		if application.AgentID == agentID {
			p = application.BasePrompt
		}
	}
	if p == "" {
		return "你是 ChatGPT, 一个由 OpenAI 训练的大型语言模型, 你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。\n"
	}
	return p + "\n"
}
