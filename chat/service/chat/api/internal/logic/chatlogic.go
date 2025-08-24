package logic

import (
	"chat/common/sensecore"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chat/common/deepseek"
	"chat/common/dify"
	"chat/common/draw"
	"chat/common/gemini"
	"chat/common/openai"
	"chat/common/plugin"
	"chat/common/redis"
	"chat/common/wecom"
	"chat/service/chat/api/internal/config"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ChatLogic struct {
	logx.Logger
	ctx            context.Context
	svcCtx         *svc.ServiceContext
	model          string
	baseHost       string
	basePrompt     string
	message        string
	isVoiceRequest bool // 标识原始请求是否为语音
}

func NewChatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatLogic {
	return &ChatLogic{
		Logger:         logx.WithContext(ctx),
		ctx:            ctx,
		svcCtx:         svcCtx,
		isVoiceRequest: false, // 初始化为非语音请求
	}
}

func (l *ChatLogic) Chat(req *types.ChatReq) (resp *types.ChatReply, err error) {

	uuidObj, err := uuid.NewUUID()
	if err != nil {
		go sendToUser(req.AgentID, req.UserID, "系统错误 会话唯一标识生成失败", l.svcCtx.Config)
		return nil, err
	}
	conversationId := uuidObj.String()

	// 去 gemini 获取数据
	if req.Channel == "gemini" {

		// gemini client
		c := gemini.NewChatClient(l.svcCtx.Config.Gemini.Key).WithHost(l.svcCtx.Config.Gemini.Host).
			WithTemperature(l.svcCtx.Config.Gemini.Temperature)
		if l.svcCtx.Config.Gemini.EnableProxy {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
				WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
				WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
		}
		// 指令匹配， 根据响应值判定是否需要去调用 gemini 接口了
		proceed, _ := l.FactoryCommend(req)
		if !proceed {
			return &types.ChatReply{
				Message: "ok",
			}, nil
		}
		if l.message != "" {
			req.MSG = l.message
		}

		// 从上下文中取出用户对话
		collection := gemini.NewUserContext(
			gemini.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)),
		).WithModel(c.Model).
			WithPrompt(l.svcCtx.Config.Gemini.Prompt).
			WithClient(c).
			WithImage(req.AgentID, req.UserID). // 为后续版本做准备，Gemini 暂时不支持图文问答展示
			Set(gemini.NewChatContent(req.MSG), "", conversationId, false)

		prompts := collection.GetChatSummary()

		fmt.Println("上下文请求信息： collection.Prompt" + collection.Prompt)
		fmt.Println(prompts)
		go func() {
			// 分段响应
			if l.svcCtx.Config.Response.Stream {
				channel := make(chan string, 100)

				go func() {
					messageText, err := c.ChatStream(prompts, channel)
					if err != nil {
						errInfo := err.Error()
						if strings.Contains(errInfo, "maximum context length") {
							errInfo += "\n 请使用 #clear 清理所有上下文"
						}
						sendToUser(req.AgentID, req.UserID, "系统错误:"+err.Error(), l.svcCtx.Config)
						return
					}
					collection.Set(gemini.NewChatContent(), messageText, conversationId, true)
					// 再去插入数据
					table := l.svcCtx.ChatModel.Chat
					_ = table.WithContext(context.Background()).Create(&model.Chat{
						AgentID:    req.AgentID,
						User:       req.UserID,
						ReqContent: req.MSG,
						ResContent: messageText,
					})
				}()

				var rs []rune
				first := true
				for {
					s, ok := <-channel
					fmt.Printf("--------接受到数据: s:%s pk:%v", s, ok)
					if !ok {
						// 数据接受完成
						if len(rs) > 0 {
							// fixed #109 延时 200ms 发送消息,避免顺序错乱
							time.Sleep(200 * time.Millisecond)
							go sendToUser(req.AgentID, req.UserID, string(rs)+"\n--------------------------------\n"+req.MSG, l.svcCtx.Config)
						}
						return
					}
					rs = append(rs, []rune(s)...)

					if first && len(rs) > 50 && strings.LastIndex(string(rs), "\n") != -1 {
						lastIndex := strings.LastIndex(string(rs), "\n")
						firstPart := string(rs)[:lastIndex]
						secondPart := string(rs)[lastIndex+1:]
						// 发送数据
						go sendToUser(req.AgentID, req.UserID, firstPart, l.svcCtx.Config)
						rs = []rune(secondPart)
						first = false
					} else if len(rs) > 200 && strings.LastIndex(string(rs), "\n") != -1 {
						lastIndex := strings.LastIndex(string(rs), "\n")
						firstPart := string(rs)[:lastIndex]
						secondPart := string(rs)[lastIndex+1:]
						go sendToUser(req.AgentID, req.UserID, firstPart, l.svcCtx.Config)
						rs = []rune(secondPart)
					}
				}
			} else {
				messageText, err := c.Chat(prompts)

				fmt.Printf("gemini resp: %v \n", messageText)
				if err != nil {
					errInfo := err.Error()
					if strings.Contains(errInfo, "maximum context length") {
						errInfo += "\n 请使用 #clear 清理所有上下文"
					}
					sendToUser(req.AgentID, req.UserID, "系统错误-gemini-resp-error:"+err.Error(), l.svcCtx.Config)
					return
				}

				// 把数据 发给微信用户
				go sendToUser(req.AgentID, req.UserID, messageText, l.svcCtx.Config)

				collection.Set(gemini.NewChatContent(), messageText, conversationId, true)

				// 再去插入数据
				table := l.svcCtx.ChatModel.Chat
				_ = table.WithContext(context.Background()).Create(&model.Chat{
					AgentID:    req.AgentID,
					User:       req.UserID,
					ReqContent: req.MSG,
					ResContent: messageText,
				})
			}
		}()
	}

	if req.Channel == "deepseek" {
		// deepseek client
		c := deepseek.NewChatClient(l.svcCtx.Config.DeepSeek.Key).WithHost(l.svcCtx.Config.DeepSeek.Host).
			WithTemperature(l.svcCtx.Config.DeepSeek.Temperature).WithModel(l.svcCtx.Config.DeepSeek.Model).
			WithDebug(l.svcCtx.Config.DeepSeek.Debug)

		if l.svcCtx.Config.DeepSeek.EnableProxy {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
				WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
				WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
		}
		// 指令匹配， 根据响应值判定是否需要调用 deepseek 接口
		proceed, _ := l.FactoryCommend(req)
		if !proceed {
			return &types.ChatReply{
				Message: "ok",
			}, nil
		}
		if l.message != "" {
			req.MSG = l.message
		}

		// 从上下文中取出用户对话数据
		collection := deepseek.NewUserContext(
			deepseek.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)),
		).WithModel(c.Model).WithClient(c).WithPrompt(l.svcCtx.Config.DeepSeek.Prompt)

		// 将当前问题加入上下文
		collection.Set(deepseek.NewChatContent(req.MSG), "", conversationId, false)

		// 获取带有上下文的完整对话历史
		prompts := collection.GetChatSummary()

		fmt.Println("上下文请求信息： collection.Prompt" + collection.Prompt)
		fmt.Println(prompts)
		go func() {
			// 分段响应
			if l.svcCtx.Config.Response.Stream {
				channel := make(chan string, 100)

				go func() {
					err := c.ChatStream(prompts, channel)
					if err != nil {
						errInfo := err.Error()
						if strings.Contains(errInfo, "maximum context length") {
							errInfo += "\n 请使用 #clear 清理所有上下文"
						}
						sendToUser(req.AgentID, req.UserID, "系统错误:"+err.Error(), l.svcCtx.Config)
						return
					}
				}()

				var rs []rune
				first := true
				var fullMessage strings.Builder
				for {
					s, ok := <-channel
					if !ok {
						// 数据接受完成
						if len(rs) > 0 {
							// fixed #109 延时 200ms 发送消息,避免顺序错乱
							time.Sleep(200 * time.Millisecond)
							go sendToUser(req.AgentID, req.UserID, string(rs)+"\n--------------------------------\n"+req.MSG, l.svcCtx.Config)
						}

						// 保存完整消息到数据库
						messageText := fullMessage.String()
						// 将回复保存到上下文
						collection.Set(deepseek.NewChatContent(""), messageText, conversationId, true)

						table := l.svcCtx.ChatModel.Chat
						_ = table.WithContext(context.Background()).Create(&model.Chat{
							AgentID:    req.AgentID,
							User:       req.UserID,
							ReqContent: req.MSG,
							ResContent: messageText,
						})
						return
					}
					rs = append(rs, []rune(s)...)
					fullMessage.WriteString(s)

					if first && len(rs) > 50 && strings.LastIndex(string(rs), "\n") != -1 {
						lastIndex := strings.LastIndex(string(rs), "\n")
						firstPart := string(rs)[:lastIndex]
						secondPart := string(rs)[lastIndex+1:]
						// 发送数据
						go sendToUser(req.AgentID, req.UserID, firstPart, l.svcCtx.Config)
						rs = []rune(secondPart)
						first = false
					} else if len(rs) > 200 && strings.LastIndex(string(rs), "\n") != -1 {
						lastIndex := strings.LastIndex(string(rs), "\n")
						firstPart := string(rs)[:lastIndex]
						secondPart := string(rs)[lastIndex+1:]
						go sendToUser(req.AgentID, req.UserID, firstPart, l.svcCtx.Config)
						rs = []rune(secondPart)
					}
				}
			} else {
				messageText, err := c.Chat(prompts)
				if err != nil {
					errInfo := err.Error()
					if strings.Contains(errInfo, "maximum context length") {
						errInfo += "\n 请使用 #clear 清理所有上下文"
					}
					sendToUser(req.AgentID, req.UserID, "系统错误:"+err.Error(), l.svcCtx.Config)
					return
				}

				// 把数据发给微信用户
				go sendToUser(req.AgentID, req.UserID, messageText, l.svcCtx.Config)

				// 将回复保存到上下文
				collection.Set(deepseek.NewChatContent(""), messageText, conversationId, true)

				// 再去插入数据
				table := l.svcCtx.ChatModel.Chat
				_ = table.WithContext(context.Background()).Create(&model.Chat{
					AgentID:    req.AgentID,
					User:       req.UserID,
					ReqContent: req.MSG,
					ResContent: messageText,
				})
			}
		}()
	}

	// sense 处理
	if req.Channel == "sense" {
		c := sensecore.NewChatClient(l.svcCtx.Config.SenseCore.Key).WithHost(l.svcCtx.Config.SenseCore.Host).
			WithTemperature(l.svcCtx.Config.SenseCore.Temperature).WithModel(l.svcCtx.Config.SenseCore.Model).
			WithDebug(l.svcCtx.Config.SenseCore.Debug)

		if l.svcCtx.Config.SenseCore.EnableProxy {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
				WithProxyAuth(l.svcCtx.Config.Proxy.Auth.Username, l.svcCtx.Config.Proxy.Auth.Password)
		}

		// 指令匹配， 根据响应值判定是否需要调用 sensecore 接口
		proceed, _ := l.FactoryCommend(req)
		if !proceed {
			return &types.ChatReply{
				Message: "ok",
			}, nil
		}

		if l.message != "" {
			req.MSG = l.message
		}

		// 从上下文中取出用户对话数据
		collection := sensecore.NewUserContext(
			sensecore.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)),
		).WithModel(c.Model).WithClient(c).WithPrompt(l.svcCtx.Config.SenseCore.Prompt)

		// 将当前问题加入上下文
		collection.Set(sensecore.NewChatContent(req.MSG), "", conversationId, false)

		// 获取带有上下文的完整对话历史
		prompts := collection.GetChatSummary()

		fmt.Println("上下文请求信息： collection.Prompt" + collection.Prompt)
		fmt.Println(prompts)
		go func() {
			// 分段响应
			if l.svcCtx.Config.Response.Stream {
				channel := make(chan string, 100)

				go func() {
					err := c.ChatStream(prompts, channel)
					if err != nil {
						errInfo := err.Error()
						if strings.Contains(errInfo, "maximum context length") {
							errInfo += "\n 请使用 #clear 清理所有上下文"
						}
						sendToUser(req.AgentID, req.UserID, "系统错误:"+err.Error(), l.svcCtx.Config)
						return
					}
				}()

				var rs []rune
				first := true
				var fullMessage strings.Builder
				for {
					s, ok := <-channel
					if !ok {
						// 数据接受完成
						if len(rs) > 0 {
							// fixed #109 延时 200ms 发送消息,避免顺序错乱
							time.Sleep(200 * time.Millisecond)
							go sendToUser(req.AgentID, req.UserID, string(rs)+"\n--------------------------------\n"+req.MSG, l.svcCtx.Config)
						}
					}
				}
			}
		}()
	}	
	}

	// 去找 openai 获取数据
	if req.Channel == "openai" {
		l.setModelName(req.AgentID).setBasePrompt(req.AgentID).setBaseHost()

		// 如果用户有自定义的配置，就使用用户的配置
		table := l.svcCtx.ChatConfigModel.ChatConfig
		configCollection, configErr := table.WithContext(context.Background()).
			Where(table.User.Eq(req.UserID)).Where(table.AgentID.Eq(req.AgentID)).
			Order(table.ID.Desc()).First()
		if configErr == nil && configCollection.ID > 0 {
			l.basePrompt = configCollection.Prompt
			l.model = configCollection.Model
		}

		// 指令匹配， 根据响应值判定是否需要去调用 openai 接口了
		proceed, _ := l.FactoryCommend(req)
		if !proceed {
			return &types.ChatReply{
				Message: "ok",
			}, nil
		}
		if l.message != "" {
			req.MSG = l.message
		}

		// openai client
		c := openai.NewChatClient(l.svcCtx.Config.OpenAi.Key).
			WithModel(l.model).
			WithBaseHost(l.baseHost).
			WithOrigin(l.svcCtx.Config.OpenAi.Origin).
			WithEngine(l.svcCtx.Config.OpenAi.Engine).
			WithMaxToken(l.svcCtx.Config.OpenAi.MaxToken).
			WithTemperature(l.svcCtx.Config.OpenAi.Temperature).
			WithTotalToken(l.svcCtx.Config.OpenAi.TotalToken)

		if l.svcCtx.Config.OpenAi.EnableProxy {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
				WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
				WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
		}

		// context
		collection := openai.NewUserContext(
			openai.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)),
		).WithModel(l.model).WithPrompt(l.basePrompt).WithClient(c).WithTimeOut(l.svcCtx.Config.Session.TimeOut)

		go func() {
			// 通过插件处理数据
			if l.svcCtx.Config.Plugins.Enable && len(l.svcCtx.Config.Plugins.List) > 0 {
				// 通过插件处理
				var p []plugin.Plugin
				for _, i2 := range l.svcCtx.Config.Plugins.List {
					p = append(p, plugin.Plugin{
						NameForModel: i2.NameForModel,
						DescModel:    i2.DescModel,
						API:          i2.API,
					})
				}
				pluginInfo, err := c.WithMaxToken(1000).WithTemperature(0).
					Chat(plugin.GetChatPluginPromptInfo(req.MSG, p))
				// 还原参数
				c.WithMaxToken(l.svcCtx.Config.OpenAi.MaxToken).WithTemperature(l.svcCtx.Config.OpenAi.Temperature)
				if err == nil {
					runPluginInfo, ok := plugin.RunPlugin(pluginInfo, p)
					if ok {
						if runPluginInfo.Wrapper == false {
							// 插件处理成功，发送给用户
							go sendToUser(req.AgentID, req.UserID, runPluginInfo.Output+"\n"+req.MSG, l.svcCtx.Config)
							return
						}
						q := fmt.Sprintf(
							"根据用户输入\n%s\n\nai决定使用%s插件\nai请求插件的信息为: %s\n通过插件获取到的响应信息为: %s\n 。请确认以上信息，如果信息中存在与你目前信息不一致的地方，请以上方%s插件提供的信息为准，比如日期... 并将其作为后续回答的依据，确认请回复 ok ,不要解释",
							req.MSG, runPluginInfo.PluginName, runPluginInfo.Input, runPluginInfo.Output, runPluginInfo.PluginName,
						)
						// 插件处理成功，存入上下文
						collection.Set(openai.NewChatContent(q), "ok", conversationId, false)
						if l.svcCtx.Config.Plugins.Debug {
							// 通知用户正在使用插件并响应结果
							go sendToUser(req.AgentID, req.UserID, fmt.Sprintf(
								"根据用户输入:\n%s\n\nai决定使用%s插件\nai请求插件的信息为: %s\nai通过插件获取到的响应信息为 %s",
								req.MSG, runPluginInfo.PluginName, runPluginInfo.Input, runPluginInfo.Output),
								l.svcCtx.Config)
						}
					}
				}
			}

			// 基于 summary 进行补充
			messageText := ""
			collection.Set(openai.NewChatContent(req.MSG), "", conversationId, false)
			prompts := collection.GetChatSummary()

			// 分段响应
			if l.svcCtx.Config.Response.Stream {
				channel := make(chan string, 100)

				go func() {
					if l.model == openai.TextModel {
						messageText, err = c.CompletionStream(prompts, channel)
					} else {
						messageText, err = c.ChatStream(prompts, channel)
					}
					if err != nil {
						errInfo := err.Error()
						if strings.Contains(errInfo, "maximum context length") {
							errInfo += "\n 请使用 #clear 清理所有上下文"
						}
						sendToUser(req.AgentID, req.UserID, "系统错误:"+err.Error(), l.svcCtx.Config)
						return
					}
					collection.Set(openai.NewChatContent(), messageText, conversationId, true)
					// 再去插入数据
					table := l.svcCtx.ChatModel.Chat
					_ = table.WithContext(context.Background()).Create(&model.Chat{
						AgentID:    req.AgentID,
						User:       req.UserID,
						ReqContent: req.MSG,
						ResContent: messageText,
					})
				}()

				var rs []rune
				first := true
				for {
					s, ok := <-channel
					if !ok {
						// 数据接受完成
						if len(rs) > 0 {
							// fixed #109 延时 200ms 发送消息,避免顺序错乱
							time.Sleep(200 * time.Millisecond)
							go sendToUser(req.AgentID, req.UserID, string(rs)+"\n--------------------------------\n"+req.MSG, l.svcCtx.Config)
						}
						return
					}
					rs = append(rs, []rune(s)...)

					if first && len(rs) > 50 && strings.Contains(s, "\n\n") {
						go sendToUser(req.AgentID, req.UserID, strings.TrimRight(string(rs), "\n\n"), l.svcCtx.Config)
						rs = []rune{}
						first = false
					} else if len(rs) > 100 && strings.Contains(s, "\n\n") {
						go sendToUser(req.AgentID, req.UserID, strings.TrimRight(string(rs), "\n\n"), l.svcCtx.Config)
						rs = []rune{}
					}
				}
			}

			// 一次性响应
			if l.model == openai.TextModel {
				messageText, err = c.Completion(prompts)
			} else {
				messageText, err = c.Chat(prompts)
			}

			if err != nil {
				errInfo := err.Error()
				if strings.Contains(errInfo, "maximum context length") {
					errInfo += "\n 请使用 #clear 清理所有上下文"
				}
				sendToUser(req.AgentID, req.UserID, "系统错误:"+err.Error(), l.svcCtx.Config)
				return
			}
			// 把数据 发给微信用户
			go sendToUser(req.AgentID, req.UserID, messageText, l.svcCtx.Config)

			collection.Set(openai.NewChatContent(), messageText, conversationId, true)

			// 再去插入数据
			table := l.svcCtx.ChatModel.Chat
			_ = table.WithContext(context.Background()).Create(&model.Chat{
				AgentID:    req.AgentID,
				User:       req.UserID,
				ReqContent: req.MSG,
				ResContent: messageText,
			})
		}()
	}

	// dify 处理
	if req.Channel == "dify" {
		c := dify.NewClient(l.svcCtx.Config.Dify.Host, l.svcCtx.Config.Dify.Key)

		// 指令匹配， 根据响应值判定是否需要调用 dify 接口
		proceed, _ := l.FactoryCommend(req)
		if !proceed {
			return &types.ChatReply{
				Message: "ok",
			}, nil
		}
		if l.message != "" {
			req.MSG = l.message
		}

		request := dify.WorkflowRequest{
			Query:        req.MSG,
			User:         req.UserID,
			ResponseMode: "streaming",
			Inputs:       map[string]any{},
		}
		if len(l.svcCtx.Config.Dify.Inputs) > 0 {
			for _, v := range l.svcCtx.Config.Dify.Inputs {
				request.Inputs[v.Key] = v.Value
			}
		}

		// 从 redis 中获取会话 ID
		cacheKey := fmt.Sprintf(redis.DifyConversationKey, req.AgentID, req.UserID)
		conversationId, err := redis.Rdb.Get(context.Background(), cacheKey).Result()
		if err == nil && conversationId != "" {
			// 如果有会话ID，使用已有的会话ID
			request.ConversationId = conversationId
		}

		go func() {
			ctx := context.Background()
			// 设置超时时间为 200 秒
			ctx, cancel := context.WithTimeout(ctx, 200*time.Second)
			defer cancel()

			// 分段响应
			if l.svcCtx.Config.Response.Stream {
				var (
					strBuilder  strings.Builder
					messageText string
					rs          []rune
					first       = true
				)

				// 创建自定义的 EventHandler
				handler := &difyEventHandler{
					logger: l.Logger,
					onStreamingResponse: func(resp dify.StreamingResponse) {
						l.Logger.Debug("Received streaming response:", resp)

						// 获取文本内容，通常在outputs中的text字段
						var textContent string
						if resp.Event == dify.EventWorkflowStarted {
							go sendToUser(req.AgentID, req.UserID, "我们已经收到了您的请求正在处理中...", l.svcCtx.Config)
							// 去将 conversation_id 存入 redis
							if resp.ConversationID != "" {
								cacheKey := fmt.Sprintf(redis.DifyConversationKey, req.AgentID, req.UserID)
								redis.Rdb.Set(context.Background(), cacheKey, resp.ConversationID, 24*time.Hour)
							}
						}
						if resp.Event == dify.EventNodeFinished {
							if resp.Data.Outputs != nil {
								if textVal, ok := resp.Data.Outputs["answer"]; ok {
									if text, ok := textVal.(string); ok {
										textContent = text
									}
								}
							}
						}

						if textContent != "" {
							rs = append(rs, []rune(textContent)...)
							strBuilder.WriteString(textContent)

							if first && len(rs) > 50 && strings.LastIndex(string(rs), "\n") != -1 {
								lastIndex := strings.LastIndex(string(rs), "\n")
								firstPart := string(rs)[:lastIndex]
								secondPart := string(rs)[lastIndex+1:]
								// 发送数据
								go sendToUser(req.AgentID, req.UserID, firstPart, l.svcCtx.Config)
								rs = []rune(secondPart)
								first = false
							} else if len(rs) > 200 && strings.LastIndex(string(rs), "\n") != -1 {
								lastIndex := strings.LastIndex(string(rs), "\n")
								firstPart := string(rs)[:lastIndex]
								secondPart := string(rs)[lastIndex+1:]
								go sendToUser(req.AgentID, req.UserID, firstPart, l.svcCtx.Config)
								rs = []rune(secondPart)
							}
						}

						// 如果是工作流结束事件，发送剩余内容
						if resp.Event == dify.EventWorkflowFinished {
							// 延时 300ms 发送消息，避免顺序错乱
							time.Sleep(300 * time.Millisecond)
							go sendToUser(req.AgentID, req.UserID, string(rs)+"\n--------------------------------\n"+req.MSG, l.svcCtx.Config)

							messageText = strBuilder.String()
							if l.svcCtx.Config.Dify.ResponseWithVoice {
								// 生成语音
								go func() {
									response, err := c.API().TextToAudio(context.Background(), messageText)
									if err == nil {
										// build file path
										filePath := fmt.Sprintf("%s/%d-%s", os.TempDir(), req.AgentID, uuidObj.String())
										// save voice
										filePath, err = dify.SaveAudioToFile(response.Audio, filePath, response.ContentType)
										if err == nil {
											go sendToUser(req.AgentID, req.UserID, "", l.svcCtx.Config, filePath)
										} else {
											l.Logger.Error("dify 生成语音失败: ", err)
										}
									} else {
										l.Logger.Error("dify 生成语音失败: ", err)
									}
								}()
							}
							// 将对话记录存储到数据库
							table := l.svcCtx.ChatModel.Chat
							_ = table.WithContext(context.Background()).Create(&model.Chat{
								AgentID:    req.AgentID,
								User:       req.UserID,
								ReqContent: req.MSG,
								ResContent: messageText,
							})
						}
					},
					onError: func(err error) {
						errInfo := err.Error()
						if strings.Contains(errInfo, "maximum context length") {
							errInfo += "\n 请使用 #clear 清理所有上下文"
						}
						sendToUser(req.AgentID, req.UserID, "系统错误:"+errInfo, l.svcCtx.Config)
					},
				}

				err := c.API().RunStreamWorkflowWithHandler(ctx, request, handler)
				if err != nil {
					errInfo := err.Error()
					if strings.Contains(errInfo, "maximum context length") {
						errInfo += "\n 请使用 #clear 清理所有上下文"
					}
					sendToUser(req.AgentID, req.UserID, "系统错误:"+errInfo, l.svcCtx.Config)
				}
			} else {
				l.Logger.Debug("dify 处理 非流式响应: ", request)
				// 非流式响应 - 需要将 WorkflowRequest 转换为 ChatMessageRequest
				chatRequest := &dify.ChatMessageRequest{
					Query:        request.Query,
					User:         request.User,
					ResponseMode: "blocking",
					Inputs:       request.Inputs,
				}

				resp, err := c.API().ChatMessages(ctx, chatRequest)
				if err != nil {
					errInfo := err.Error()
					if strings.Contains(errInfo, "maximum context length") {
						errInfo += "\n 请使用 #clear 清理所有上下文"
					}
					sendToUser(req.AgentID, req.UserID, "系统错误:"+errInfo, l.svcCtx.Config)
					return
				}

				messageText := resp.Answer

				// 把数据发给微信用户
				go sendToUser(req.AgentID, req.UserID, messageText, l.svcCtx.Config)

				// 再去插入数据
				table := l.svcCtx.ChatModel.Chat
				_ = table.WithContext(context.Background()).Create(&model.Chat{
					AgentID:    req.AgentID,
					User:       req.UserID,
					ReqContent: req.MSG,
					ResContent: messageText,
				})
				l.Logger.Debug("dify 处理完成: ", messageText)
			}
		}()
	}

	if req.Channel == "wecom" {
		sendToUser(req.AgentID, req.UserID, req.MSG, l.svcCtx.Config)
	}

	return &types.ChatReply{
		Message: "ok",
	}, nil
}

func (l *ChatLogic) setBaseHost() (ls *ChatLogic) {
	if l.svcCtx.Config.OpenAi.Host == "" {
		l.svcCtx.Config.OpenAi.Host = "https://api.openai.com"
	}
	l.baseHost = l.svcCtx.Config.OpenAi.Host
	return l
}

func (l *ChatLogic) setModelName(agentID int64) (ls *ChatLogic) {
	m := openai.ChatModel
	for _, application := range l.svcCtx.Config.WeCom.MultipleApplication {
		if application.AgentID == agentID {
			m = application.Model
		}
	}
	// 兼容大小写问题 #issues/66
	l.model = strings.ToLower(m)
	return l
}

func (l *ChatLogic) setBasePrompt(agentID int64) (ls *ChatLogic) {
	p := ""
	for _, application := range l.svcCtx.Config.WeCom.MultipleApplication {
		if application.AgentID == agentID {
			p = application.BasePrompt
		}
	}
	if p == "" {
		p = "你是 ChatGPT, 一个由 OpenAI 训练的大型语言模型, 你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。\n"
	}
	l.basePrompt = p
	return l
}

func (l *ChatLogic) FactoryCommend(req *types.ChatReq) (proceed bool, err error) {
	template := make(map[string]TemplateData)
	//当 message 以 # 开头时，表示是特殊指令
	if !strings.HasPrefix(req.MSG, "#") {
		return true, nil
	}

	template["#clear"] = CommendClear{}
	template["#session"] = CommendSession{}
	template["#config_prompt:"] = CommendConfigPrompt{}
	template["#config_model:"] = CommendConfigModel{}
	template["#config_clear"] = CommendConfigClear{}
	template["#help"] = CommendHelp{}
	template["#image"] = CommendImage{}
	template["#voice"] = CommendVoice{}
	template["#draw"] = CommendDraw{}
	template["#prompt:list"] = CommendPromptList{}
	template["#prompt:set:"] = CommendPromptSet{}
	template["#system"] = CommendSystem{}
	template["#welcome"] = CommendWelcome{}
	template["#about"] = CommendAbout{}
	//template["#usage"] = CommendUsage{}
	template["#plugin"] = CommendPlugin{}

	for s, data := range template {
		if strings.HasPrefix(req.MSG, s) {
			proceed = data.exec(l, req)
			return proceed, nil
		}
	}

	return true, nil
}

// 发送消息给用户
func sendToUser(agentID any, userID, msg string, config config.Config, file ...string) {
	// 根据 agentID 的类型 执行不同的方法
	switch agentID.(type) {
	case int64:
		corpSecret := ""
		for _, application := range config.WeCom.MultipleApplication {
			if application.AgentID == agentID.(int64) {
				corpSecret = application.AgentSecret
			}
		}
		wecom.SendToWeComUser(agentID.(int64), userID, msg, corpSecret, file...)
	case string:
		// 对客户消息格式进行处理
		processedMsg := processMarkdownText(msg)
		wecom.SendCustomerChatMessage(agentID.(string), userID, processedMsg, file...)
	}
}

// processMarkdownText 处理消息中的Markdown格式
// 去除markdown标记和调整换行符
func processMarkdownText(msg string) string {
	if msg == "" {
		return msg
	}

	// 替换连续多个换行为单个换行
	re := regexp.MustCompile(`\n{2,}`)
	msg = re.ReplaceAllString(msg, "\n")

	// 去除Markdown加粗标记 **text**
	boldRe := regexp.MustCompile(`\*\*(.*?)\*\*`)
	msg = boldRe.ReplaceAllString(msg, "$1")

	// 去除Markdown斜体标记 *text* 或 _text_
	italicRe := regexp.MustCompile(`([*_])(.*?)([*_])`)
	msg = italicRe.ReplaceAllString(msg, "$2")

	// 去除Markdown标题标记 # text
	headerRe := regexp.MustCompile(`(?m)^#+\s+(.*?)$`)
	msg = headerRe.ReplaceAllString(msg, "$1")

	// 去除Markdown链接标记 [text](url)
	linkRe := regexp.MustCompile(`\[(.*?)]\(.*?\)`)
	msg = linkRe.ReplaceAllString(msg, "$1")

	// 去除Markdown代码块标记 ```code```，保留内部内容
	codeBlockStart := regexp.MustCompile("(?ms)```.*?\n")
	msg = codeBlockStart.ReplaceAllString(msg, "")
	codeBlockEnd := regexp.MustCompile("(?ms)```")
	msg = codeBlockEnd.ReplaceAllString(msg, "")

	// 去除Markdown行内代码标记 `code`
	inlineCodeRe := regexp.MustCompile("`(.*?)`")
	msg = inlineCodeRe.ReplaceAllString(msg, "$1")

	// 去除Markdown列表标记 - text 或 * text 或 1. text
	listRe := regexp.MustCompile(`(?m)^\s*[-*]\s+(.*?)$`)
	msg = listRe.ReplaceAllString(msg, "$1")
	orderedListRe := regexp.MustCompile(`(?m)^\s*\d+\.\s+(.*?)$`)
	msg = orderedListRe.ReplaceAllString(msg, "$1")

	return strings.TrimSpace(msg)
}

type TemplateData interface {
	exec(svcCtx *ChatLogic, req *types.ChatReq) (proceed bool)
}

// CommendClear 清除用户的所有对话数据
type CommendClear struct{}

func (p CommendClear) exec(l *ChatLogic, req *types.ChatReq) bool {
	openai.NewUserContext(
		openai.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)),
	).Clear()
	if req.Channel == "dify" {
		cacheKey := fmt.Sprintf(redis.DifyConversationKey, req.AgentID, req.UserID)
		redis.Rdb.Del(context.Background(), cacheKey)
	}
	sendToUser(req.AgentID, req.UserID, "当前会话清理完成，来开始新一轮的chat吧", l.svcCtx.Config)
	return false
}

// CommendHelp 查看所有指令
type CommendHelp struct{}

func (p CommendHelp) exec(l *ChatLogic, req *types.ChatReq) bool {
	tips := fmt.Sprintf(
		"支持指令：\n\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n",
		"基础模块🕹️\n\n#help       查看所有指令",
		"#system 查看会话系统信息",
		"#usage 查看额度使用情况\n#usage:sk-xxx 查看指定 key 的使用情况",
		"#clear 清空当前会话的数据",
		"\n会话设置🦄\n\n#config_prompt:xxx，如程序员的小助手",
		"#config_model:xxx，如gpt-3.5-turbo-16k",
		"#config_clear 初始化对话设置",
		"#prompt:list 查看所有支持的预定义角色",
		"#prompt:set:xx 如 24(诗人)，角色应用",
		"\n会话控制🚀\n",
		"#session:start 开启新的会话",
		"#session:list    查看所有会话\n#session:clear 清空所有会话",
		"#session:export:json 导出当前会话数据为json\n#session:export:txt 导出当前会话数据为txt",
		"#session:exchange:xxx 切换指定会话",
		"\n绘图🎨\n",
		"#draw:xxx 按照指定 prompt 进行绘画",
		"\n插件🛒\n",
		"#plugin:list 查看所有插件",
		//"#plugin:enable:xxx 启用指定插件\n",
		//"#plugin:disable:xxx 禁用指定插件\n",
	)
	sendToUser(req.AgentID, req.UserID, tips, l.svcCtx.Config)
	return false
}

type CommendSystem struct{}

func (p CommendSystem) exec(l *ChatLogic, req *types.ChatReq) bool {
	// 是否开启插件
	pluginStatus := "关闭"
	if l.svcCtx.Config.Plugins.Enable {
		pluginStatus = "开启"
	}
	tips := fmt.Sprintf(
		"系统信息\n系统版本为：%s \n\nmodel 版本为：%s \n\n系统基础设定：%s \n\n插件是否开启：%s ",
		l.svcCtx.Config.SystemVersion,
		l.model,
		l.basePrompt,
		pluginStatus,
	)
	sendToUser(req.AgentID, req.UserID, tips, l.svcCtx.Config)
	return false
}

type CommendConfigPrompt struct{}

func (p CommendConfigPrompt) exec(l *ChatLogic, req *types.ChatReq) bool {
	// #config_prompt:您的设置 如 程序员的小助手\n
	// 处理 msg
	msg := strings.Replace(req.MSG, "#config_prompt:", "", -1)
	if msg == "" {
		sendToUser(req.AgentID, req.UserID, "请输入完整的设置 如：#config_prompt:程序员的小助手", l.svcCtx.Config)
		return false
	}
	// 去数据库新增用户的对话配置
	chatConfig := model.ChatConfig{
		AgentID: req.AgentID,
		User:    req.UserID,
		Prompt:  msg,
		Model:   l.model,
	}
	table := l.svcCtx.ChatConfigModel.ChatConfig
	configErr := table.WithContext(context.Background()).Create(&chatConfig)

	if configErr != nil {
		sendToUser(req.AgentID, req.UserID, "设置失败,请稍后再试~", l.svcCtx.Config)
		return false
	}

	sendToUser(req.AgentID, req.UserID, "设置成功，您目前的对话配置如下：\n prompt: "+msg+"\n model: "+l.model, l.svcCtx.Config)
	return false
}

type CommendConfigModel struct{}

func (p CommendConfigModel) exec(l *ChatLogic, req *types.ChatReq) bool {
	msg := strings.Trim(strings.Replace(req.MSG, "#config_model:", "", -1), " ")

	if msg == "" {
		sendToUser(req.AgentID, req.UserID, "请输入完整的设置 如：\n#config_model:gpt-3.5-turbo", l.svcCtx.Config)
		return false
	}

	// 去数据库新增用户的对话配置
	chatConfig := model.ChatConfig{
		AgentID: req.AgentID,
		User:    req.UserID,
		Prompt:  l.basePrompt,
		Model:   msg,
	}
	table := l.svcCtx.ChatConfigModel.ChatConfig
	configErr := table.WithContext(context.Background()).Create(&chatConfig)

	if configErr != nil {
		sendToUser(req.AgentID, req.UserID, "设置失败,请稍后再试~", l.svcCtx.Config)
		return false
	}

	sendToUser(req.AgentID, req.UserID, "设置成功，您目前的对话配置如下：\n prompt: "+l.basePrompt+"\n model: "+msg, l.svcCtx.Config)
	return false
}

type CommendConfigClear struct{}

func (p CommendConfigClear) exec(l *ChatLogic, req *types.ChatReq) bool {
	// 去数据库删除 用户的所有对话配置
	table := l.svcCtx.ChatConfigModel.ChatConfig
	collection, _ := table.WithContext(context.Background()).Where(table.User.Eq(req.UserID)).
		Where(table.AgentID.Eq(req.AgentID)).Find()
	for _, val := range collection {
		_, _ = table.WithContext(context.Background()).Where(table.ID.Eq(val.ID)).Delete()
	}
	sendToUser(req.AgentID, req.UserID, "对话设置已恢复初始化", l.svcCtx.Config)
	return false
}

type CommendAbout struct{}

func (p CommendAbout) exec(l *ChatLogic, req *types.ChatReq) bool {
	sendToUser(req.AgentID, req.UserID, "https://github.com/whyiyhw/chatgpt-wechat", l.svcCtx.Config)
	return false
}

type CommendWelcome struct{}

func (p CommendWelcome) exec(l *ChatLogic, req *types.ChatReq) bool {
	cacheKey := fmt.Sprintf(redis.WelcomeCacheKey, req.AgentID, req.UserID)
	if _, err := redis.Rdb.Get(context.Background(), cacheKey).Result(); err == nil {
		return false
	}
	welcome := ""
	for _, s := range l.svcCtx.Config.WeCom.MultipleApplication {
		if s.AgentID == req.AgentID {
			welcome = s.Welcome
		}
	}
	sendToUser(req.AgentID, req.UserID, welcome, l.svcCtx.Config)
	_, err := redis.Rdb.SetEx(context.Background(), cacheKey, "1", 24*15*time.Hour).Result()
	if err != nil {
		fmt.Println("welcome2:" + err.Error())
	}
	return false
}

type CommendImage struct{}

func (p CommendImage) exec(l *ChatLogic, req *types.ChatReq) bool {
	// #image:https://www.baidu.com/img/bd_logo1.png
	msg := strings.Replace(req.MSG, "#image:", "", -1)
	if msg == "" {
		sendToUser(req.AgentID, req.UserID, "请输入完整的设置 如：#image:https://www.google.com/img/bd_logo1.png", l.svcCtx.Config)
		return false
	}
	// 中间思路，请求进行图片识别
	c := gemini.NewChatClient(l.svcCtx.Config.Gemini.Key).WithHost(l.svcCtx.Config.Gemini.Host).
		WithTemperature(l.svcCtx.Config.Gemini.Temperature).WithModel(gemini.VisionModel)
	if l.svcCtx.Config.Gemini.EnableProxy {
		c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
			WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
			WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
	}
	var parseImage []gemini.ChatModelMessage
	// 将 图片 转成 base64
	base64Data, mime, err := gemini.GetImageContent(msg)
	if err != nil {
		sendToUser(req.AgentID, req.UserID, "图片解析失败:"+err.Error(), l.svcCtx.Config)
		return false
	}
	sendToUser(req.AgentID, req.UserID, "好的收到了您的图片，正在识别中~", l.svcCtx.Config)
	result, err := c.Chat(append(parseImage, gemini.ChatModelMessage{
		Role:    gemini.UserRole,
		Content: gemini.NewChatContent(base64Data, mime),
	}, gemini.ChatModelMessage{
		Role:    gemini.UserRole,
		Content: gemini.NewChatContent("你能详细描述图片中的内容吗？"),
	}))
	if err != nil {
		sendToUser(req.AgentID, req.UserID, "图片识别失败:"+err.Error(), l.svcCtx.Config)
		return false
	}

	sendToUser(req.AgentID, req.UserID, "图片识别完成:\n\n"+result, l.svcCtx.Config)
	// 将其存入 上下文
	gemini.NewUserContext(
		gemini.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)),
	).WithModel(c.Model).
		WithPrompt(l.svcCtx.Config.Gemini.Prompt).
		WithClient(c).
		Set(
			gemini.NewChatContent(
				"我向你描述一副图片的内容如下：\n\n"+result),
			"收到,我理解了您的图片！",
			"",
			true,
		)
	return false
}

type CommendPromptList struct{}

func (p CommendPromptList) exec(l *ChatLogic, req *types.ChatReq) bool {
	// #prompt:list
	// 去数据库获取用户的所有prompt
	e := l.svcCtx.PromptConfigModel.PromptConfig
	collection, _ := e.WithContext(context.Background()).Where(e.ID.Gt(1)).Find()
	var prompts []string
	for _, val := range collection {
		prompts = append(prompts, fmt.Sprintf("%s:%d", val.Key, val.ID))
	}
	sendToUser(req.AgentID, req.UserID, "您的prompt如下：\n"+strings.Join(prompts, "\n"), l.svcCtx.Config)
	return false
}

type CommendPromptSet struct{}

func (p CommendPromptSet) exec(l *ChatLogic, req *types.ChatReq) bool {
	// #prompt:您的设置 如：您好，我是小助手，很高兴为您服务\n
	msg := strings.Trim(strings.Replace(req.MSG, "#prompt:set:", "", -1), " ")

	if msg == "" {
		sendToUser(req.AgentID, req.UserID, "请输入完整的设置 如：\n#prompt:set:24\n", l.svcCtx.Config)
		return false
	}
	// msg 转 int64
	mId, err := strconv.ParseInt(msg, 10, 64)
	if err != nil {
		sendToUser(req.AgentID, req.UserID, "请输入完整的设置 如：\n#prompt:set:24\n", l.svcCtx.Config)
		return false
	}
	//去根据用户输入的prompt去数据库查询是否存在
	e := l.svcCtx.PromptConfigModel.PromptConfig
	prompt, _err := e.WithContext(context.Background()).Where(e.ID.Eq(mId)).First()
	switch {
	case errors.Is(_err, gorm.ErrRecordNotFound):
		sendToUser(req.AgentID, req.UserID, "此 prompt 不存在，请确认后再试", l.svcCtx.Config)
	case _err == nil:
		// 去数据库新增用户的对话配置
		chatConfig := model.ChatConfig{
			AgentID: req.AgentID,
			User:    req.UserID,
			Prompt:  prompt.Value,
			Model:   l.model,
		}
		table := l.svcCtx.ChatConfigModel.ChatConfig
		configErr := table.WithContext(context.Background()).Create(&chatConfig)

		if configErr != nil {
			sendToUser(req.AgentID, req.UserID, msg+"设置失败:"+configErr.Error(), l.svcCtx.Config)
			return false
		}
		sendToUser(req.AgentID, req.UserID, "设置成功，您目前的对话配置如下：\n prompt: "+prompt.Value+"\n model: "+l.model, l.svcCtx.Config)
	default:
		sendToUser(req.AgentID, req.UserID, "设置失败, prompt 查询失败"+_err.Error(), l.svcCtx.Config)
	}
	return false
}

type CommendVoice struct{}

func (p CommendVoice) exec(l *ChatLogic, req *types.ChatReq) bool {
	msg := strings.Replace(req.MSG, "#voice:", "", -1)
	if msg == "" {
		sendToUser(req.AgentID, req.UserID, "未能读取到音频信息", l.svcCtx.Config)
		return false
	}

	// 设置标志，表示这是一个语音请求
	l.isVoiceRequest = true

	if req.Channel == "dify" {
		text, err := dify.NewClient(l.svcCtx.Config.Dify.Host, l.svcCtx.Config.Dify.Key).API().AudioToText(context.Background(), msg)
		if err != nil {
			sendToUser(req.AgentID, req.UserID, "音频信息转换错误:"+err.Error(), l.svcCtx.Config, msg)
			return false
		}
		if text == "" {
			sendToUser(req.AgentID, req.UserID, "音频信息转换为空", l.svcCtx.Config)
			return false
		}
		// 语音识别成功
		sendToUser(req.AgentID, req.UserID, "语音识别成功:\n\n"+text, l.svcCtx.Config)

		l.message = text
		return true
	}

	c := openai.NewChatClient(l.svcCtx.Config.OpenAi.Key).
		WithModel(l.model).
		WithBaseHost(l.svcCtx.Config.OpenAi.Host).
		WithOrigin(l.svcCtx.Config.OpenAi.Origin).
		WithEngine(l.svcCtx.Config.OpenAi.Engine)

	if l.svcCtx.Config.OpenAi.EnableProxy {
		c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
			WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
			WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
	}

	var cli openai.Speaker
	if l.svcCtx.Config.Speaker.Company == "" {
		l.svcCtx.Config.Speaker.Company = "openai"
	}
	switch l.svcCtx.Config.Speaker.Company {
	case "openai":
		logx.Info("使用openai音频转换")
		cli = c
	default:
		sendToUser(req.AgentID, req.UserID, "系统错误:未知的音频转换服务商", l.svcCtx.Config)
		return false
	}
	fmt.Println(cli)
	txt, err := cli.SpeakToTxt(msg)
	if err != nil {
		sendToUser(req.AgentID, req.UserID, "音频信息转换错误:"+err.Error(), l.svcCtx.Config)
		return false
	}
	if txt == "" {
		sendToUser(req.AgentID, req.UserID, "音频信息转换为空", l.svcCtx.Config)
		return false
	}
	// 语音识别成功
	sendToUser(req.AgentID, req.UserID, "语音识别成功:\n\n"+txt, l.svcCtx.Config)

	l.message = txt
	return true
}

type CommendSession struct{}

func (p CommendSession) exec(l *ChatLogic, req *types.ChatReq) bool {
	if strings.HasPrefix(req.MSG, "#session:start") {

		openai.NewSession(openai.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)))

		sendToUser(req.AgentID, req.UserID, "已为您保存上下文，新的会话已开启 \n您可以通过 #session:list 查看所有会话", l.svcCtx.Config)
		return false
	}

	if req.MSG == "#session:list" {
		sessions := openai.GetSessions(openai.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)))
		var sessionList []string
		defaultSessionKey := openai.NewUserContext(openai.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10))).SessionKey
		for _, session := range sessions {
			if session == defaultSessionKey {
				sessionList = append(sessionList, fmt.Sprintf("%s:%s(当前)", "#session:list", session))
			} else {
				sessionList = append(sessionList, fmt.Sprintf("%s:%s", "#session:list", session))
			}
		}
		sendToUser(req.AgentID, req.UserID, "您的会话列表如下：\n"+strings.Join(sessionList, "\n"), l.svcCtx.Config)
		return false
	}

	if req.MSG == "#session:clear" {
		openai.ClearSessions(openai.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)))
		sendToUser(req.AgentID, req.UserID, "所有会话已清除", l.svcCtx.Config)
		return false
	}

	// #session:list:xxx
	if strings.HasPrefix(req.MSG, "#session:exchange:") {
		session := strings.Replace(req.MSG, "#session:exchange:", "", -1)
		err := openai.SetSession(openai.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)), session)
		if err != nil {
			sendToUser(req.AgentID, req.UserID, "会话切换失败 \nerr:"+err.Error(), l.svcCtx.Config)
			return false
		}
		sendToUser(req.AgentID, req.UserID, "已为您切换会话", l.svcCtx.Config)
		return false
	}

	if strings.HasPrefix(req.MSG, "#session:export:") {
		prefix := strings.Replace(req.MSG, "#session:export:", "", -1)
		// context
		path, err := openai.NewUserContext(
			openai.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)),
		).SaveAllChatMessage(prefix)
		if err != nil {
			sendToUser(req.AgentID, req.UserID, "导出失败 \nerr:"+err.Error(), l.svcCtx.Config)
			return false
		}
		sendToUser(req.AgentID, req.UserID, "", l.svcCtx.Config, path)
		return false
	}

	sendToUser(req.AgentID, req.UserID, "未知的命令，您可以通过 \n#help \n查看帮助", l.svcCtx.Config)
	return false
}

type CommendDraw struct{}

func (p CommendDraw) exec(l *ChatLogic, req *types.ChatReq) bool {
	if strings.HasPrefix(req.MSG, "#draw:") {
		prompt := strings.Replace(req.MSG, "#draw:", "", -1)
		if l.svcCtx.Config.Draw.Enable {
			go func() {
				var d draw.Draw
				if l.svcCtx.Config.Draw.Company == draw.SD {
					d = draw.NewSdDraw(
						strings.TrimRight(l.svcCtx.Config.Draw.StableDiffusion.Host, "/"),
						l.svcCtx.Config.Draw.StableDiffusion.Auth.Username,
						l.svcCtx.Config.Draw.StableDiffusion.Auth.Password,
					)
				} else if l.svcCtx.Config.Draw.Company == draw.OPENAI {
					openaiDraw := openai.NewOpenaiDraw(
						l.svcCtx.Config.Draw.OpenAi.Host, l.svcCtx.Config.Draw.OpenAi.Key).
						WithOrigin(l.svcCtx.Config.Draw.OpenAi.Origin).
						WithEngine(l.svcCtx.Config.Draw.OpenAi.Engine)
					if l.svcCtx.Config.Draw.OpenAi.EnableProxy {
						openaiDraw.WithHttpProxy(l.svcCtx.Config.Proxy.Http).
							WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
							WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
							WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
					}
					d = openaiDraw
				} else {
					sendToUser(req.AgentID, req.UserID, "系统错误:未知的绘画服务商", l.svcCtx.Config)
					return
				}
				// 如果 prompt 中包含中文，将 中文 prompt 通过 openai 转换为英文
				// 如何判断 prompt 中是否包含中文？
				// 通过正则匹配，如果匹配到中文，则进行转换
				if regexp.MustCompile("[\u4e00-\u9fa5]").MatchString(prompt) {
					// openai client
					c := openai.NewChatClient(l.svcCtx.Config.OpenAi.Key).
						WithModel(l.model).
						WithBaseHost(l.baseHost).
						WithOrigin(l.svcCtx.Config.OpenAi.Origin).
						WithEngine(l.svcCtx.Config.OpenAi.Engine).
						WithMaxToken(2000).
						WithTemperature(0).
						WithTotalToken(l.svcCtx.Config.OpenAi.TotalToken)

					if l.svcCtx.Config.OpenAi.EnableProxy {
						c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
							WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
							WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
					}
					// 支持自定义 翻译 prompt
					translatePrompt := ""
					if l.svcCtx.Config.Draw.ZhCn2Prompt != "" {
						translatePrompt = l.svcCtx.Config.Draw.ZhCn2Prompt + "\n" + prompt
					} else {
						translatePrompt = fmt.Sprintf(draw.TranslatePrompt, prompt)
					}
					changedPrompt, err := c.Completion([]openai.ChatModelMessage{
						{
							Role:    openai.UserRole,
							Content: openai.NewChatContent(translatePrompt),
						},
					})
					if err != nil {
						sendToUser(req.AgentID, req.UserID, "系统错误:关键词转为绘画 prompt 失败"+err.Error(), l.svcCtx.Config)
						return
					}
					// 去掉\n\n
					prompt = strings.Replace(changedPrompt, "\n\n", "", -1)
				}

				// 创建一个 channel 用于接收绘画结果
				ch := make(chan string)

				// 什么时候关闭 channel？ 当收到的结果为 "stop" ，或者15分钟超时
				go func() {
					for {
						select {
						case path := <-ch:
							if path == "stop" {
								close(ch)
								return
							} else if path == "start" {
								sendToUser(req.AgentID, req.UserID, "正在绘画中...", l.svcCtx.Config)
							} else {
								sendToUser(req.AgentID, req.UserID, "", l.svcCtx.Config, path)
							}
						case <-time.After(15 * time.Minute):
							sendToUser(req.AgentID, req.UserID, "绘画请求超时", l.svcCtx.Config)
							close(ch)
							return
						}
					}
				}()

				err := d.Txt2Img(prompt, ch)
				if err != nil {
					sendToUser(req.AgentID, req.UserID, "绘画失败:"+err.Error(), l.svcCtx.Config)
					ch <- "stop"
					return
				}
			}()
			return false
		}
	}
	sendToUser(req.AgentID, req.UserID, "未知的命令，您可以通过 \n#help \n查看帮助", l.svcCtx.Config)
	return false
}

type CommendPlugin struct{}

func (p CommendPlugin) exec(l *ChatLogic, req *types.ChatReq) bool {
	if strings.HasPrefix(req.MSG, "#plugin") {
		if strings.HasPrefix(req.MSG, "#plugin:list") {
			var pluginStr string
			if l.svcCtx.Config.Plugins.Debug {
				pluginStr = "调试模式：开启 \n"
			} else {
				pluginStr = "调试模式：关闭 \n"
			}
			if l.svcCtx.Config.Plugins.Enable {
				for _, plus := range l.svcCtx.Config.Plugins.List {
					status := "禁用"
					if plus.Enable {
						status = "启用"
					}
					pluginStr += fmt.Sprintf(
						"\n插件名称：%s\n插件描述：%s\n插件状态：%s\n", plus.NameForHuman, plus.DescForHuman, status,
					)
				}
			} else {
				pluginStr = "暂无"
			}
			sendToUser(req.AgentID, req.UserID, fmt.Sprintf("当前可用的插件列表：\n%s", pluginStr), l.svcCtx.Config)
			return false
		}
	}
	return true
}

// difyEventHandler 实现 EventHandler 接口
type difyEventHandler struct {
	logger              logx.Logger
	onStreamingResponse func(dify.StreamingResponse)
	onTTSMessage        func(dify.TTSMessage)
	onError             func(error)
}

func (h *difyEventHandler) HandleStreamingResponse(resp dify.StreamingResponse) {
	if h.onStreamingResponse != nil {
		h.onStreamingResponse(resp)
	}
}

func (h *difyEventHandler) HandleTTSMessage(msg dify.TTSMessage) {
	if h.onTTSMessage != nil {
		h.onTTSMessage(msg)
	}
}
