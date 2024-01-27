package logic

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"chat/common/gemini"
	"chat/common/milvus"
	"chat/common/openai"
	"chat/common/plugin"
	"chat/common/redis"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
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

	l.setModelName().setBasePrompt().setBaseHost()

	// 确认消息没有被处理过
	table := l.svcCtx.ChatModel.Chat
	_, err = table.WithContext(l.ctx).
		Where(table.MessageID.Eq(req.MsgID)).Where(table.User.Eq(req.CustomerID)).First()
	// 消息已处理 或者 查询有问题
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		return &types.CustomerChatReply{
			Message: "ok",
		}, nil
	}

	// 指令匹配， 根据响应值判定是否需要去调用 openai 接口了
	proceed, _ := l.FactoryCommend(req)
	if !proceed {
		return &types.CustomerChatReply{
			Message: "ok",
		}, nil
	}
	if l.message != "" {
		req.Msg = l.message
	}
	company := l.svcCtx.Config.ModelProvider.Company
	modelName := ""
	var temperature float32
	// 找到 客服 对应的应用机器人
	botCustomerTable := l.svcCtx.ChatModel.BotsWithCustom
	botCustomer, botCustomerSelectErr := botCustomerTable.WithContext(l.ctx).Where(botCustomerTable.OpenKfID.Eq(req.OpenKfID)).First()
	if botCustomerSelectErr == nil {
		// 去找到 bot 机器人对应的model 配置
		botWithModelTable := l.svcCtx.ChatModel.BotsWithModel
		// 找到第一个配置
		firstModel, selectModelErr := botWithModelTable.WithContext(l.ctx).
			Where(botWithModelTable.BotID.Eq(botCustomer.BotID)).
			First()
		if selectModelErr == nil {
			company = firstModel.ModelType
			modelName = firstModel.ModelName
			temperature = float32(firstModel.Temperature)
		}
	} else {
		if company == "openai" {
			modelName = l.model
			temperature = l.svcCtx.Config.OpenAi.Temperature
		} else {
			modelName = l.svcCtx.Config.Gemini.Model
			temperature = l.svcCtx.Config.Gemini.Temperature
		}
	}

	uuidObj, uuidErr := uuid.NewUUID()
	if uuidErr != nil {
		go sendToUser(req.OpenKfID, req.CustomerID, "系统错误 会话唯一标识生成失败", l.svcCtx.Config)
	}
	conversationId := uuidObj.String()

	// gemini api
	if company == "gemini" {
		// gemini client
		c := gemini.NewChatClient(l.svcCtx.Config.Gemini.Key).WithHost(l.svcCtx.Config.Gemini.Host).
			WithTemperature(temperature)
		if l.svcCtx.Config.Gemini.EnableProxy {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
				WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
				WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
		}

		// 如果绑定了bot，那就使用bot 的 prompt 跟 各种其它设定
		botWithCustomTable := l.svcCtx.ChatModel.BotsWithCustom
		first, err := botWithCustomTable.WithContext(l.ctx).Where(botWithCustomTable.OpenKfID.Eq(req.OpenKfID)).First()
		if err == nil {
			botTable := l.svcCtx.ChatModel.Bot
			bot, err := botTable.WithContext(l.ctx).Where(botTable.ID.Eq(first.BotID)).First()
			if err == nil {
				botPromptTable := l.svcCtx.ChatModel.BotsPrompt
				botPrompt, err := botPromptTable.WithContext(l.ctx).Where(botPromptTable.BotID.Eq(bot.ID)).First()
				if err == nil {
					l.svcCtx.Config.Gemini.Prompt = botPrompt.Prompt
				}
			}
		}

		// 从上下文中取出用户对话
		collection := gemini.NewUserContext(
			gemini.GetUserUniqueID(req.CustomerID, req.OpenKfID),
		).WithModel(modelName).WithPrompt(l.svcCtx.Config.Gemini.Prompt).WithClient(c).
			//WithImage(req.OpenKfID, req.CustomerID). // 为后续版本做准备，Gemini 暂时不支持图文问答展示
			Set(gemini.NewChatContent(req.Msg), "", conversationId, false)

		prompts := collection.GetChatSummary()

		fmt.Println("上下文请求信息：")
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
						sendToUser(req.OpenKfID, req.CustomerID, "系统错误:"+err.Error(), l.svcCtx.Config)
						return
					}
					collection.Set(gemini.NewChatContent(), messageText, conversationId, true)
					// 再去插入数据
					table := l.svcCtx.ChatModel.Chat
					_ = table.WithContext(context.Background()).Create(&model.Chat{
						User:       req.CustomerID,
						OpenKfID:   req.OpenKfID,
						MessageID:  req.MsgID,
						ReqContent: req.Msg,
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
							go sendToUser(req.OpenKfID, req.CustomerID, string(rs)+
								"\n--------------------------------\n"+req.Msg, l.svcCtx.Config,
							)
						}
						return
					}
					rs = append(rs, []rune(s)...)

					if first && len(rs) > 50 && strings.LastIndex(string(rs), "\n") != -1 {
						lastIndex := strings.LastIndex(string(rs), "\n")
						firstPart := string(rs)[:lastIndex]
						secondPart := string(rs)[lastIndex+1:]
						// 发送数据
						go sendToUser(req.OpenKfID, req.CustomerID, firstPart, l.svcCtx.Config)
						rs = []rune(secondPart)
						first = false
					} else if len(rs) > 200 && strings.LastIndex(string(rs), "\n") != -1 {
						lastIndex := strings.LastIndex(string(rs), "\n")
						firstPart := string(rs)[:lastIndex]
						secondPart := string(rs)[lastIndex+1:]
						go sendToUser(req.OpenKfID, req.CustomerID, firstPart, l.svcCtx.Config)
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
					sendToUser(req.OpenKfID, req.CustomerID, "系统错误-gemini-resp-error:"+err.Error(), l.svcCtx.Config)
					return
				}

				// 把数据 发给微信用户
				go sendToUser(req.OpenKfID, req.CustomerID, messageText, l.svcCtx.Config)

				collection.Set(gemini.NewChatContent(), messageText, conversationId, true)

				// 再去插入数据
				table := l.svcCtx.ChatModel.Chat
				_ = table.WithContext(context.Background()).Create(&model.Chat{
					User:       req.CustomerID,
					OpenKfID:   req.OpenKfID,
					MessageID:  req.MsgID,
					ReqContent: req.Msg,
					ResContent: messageText,
				})
			}
		}()
	} else {
		// openai client
		c := openai.NewChatClient(l.svcCtx.Config.OpenAi.Key).
			WithModel(modelName).
			WithTemperature(temperature).
			WithBaseHost(l.baseHost).
			WithOrigin(l.svcCtx.Config.OpenAi.Origin).
			WithEngine(l.svcCtx.Config.OpenAi.Engine)
		if l.svcCtx.Config.OpenAi.EnableProxy {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
				WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
				WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
		}

		// 如果绑定了bot，那就使用bot 的 prompt 跟 各种其它设定
		botWithCustomTable := l.svcCtx.ChatModel.BotsWithCustom
		first, err := botWithCustomTable.WithContext(l.ctx).Where(botWithCustomTable.OpenKfID.Eq(req.OpenKfID)).First()
		if err == nil {
			botTable := l.svcCtx.ChatModel.Bot
			bot, err := botTable.WithContext(l.ctx).Where(botTable.ID.Eq(first.BotID)).First()
			if err == nil {
				botPromptTable := l.svcCtx.ChatModel.BotsPrompt
				botPrompt, err := botPromptTable.WithContext(l.ctx).Where(botPromptTable.BotID.Eq(bot.ID)).First()
				if err == nil {
					l.basePrompt = botPrompt.Prompt
				}
			}
		}
		// context
		collection := openai.NewUserContext(
			openai.GetUserUniqueID(req.CustomerID, req.OpenKfID),
		).WithModel(modelName).WithPrompt(l.basePrompt).WithClient(c).WithTimeOut(l.svcCtx.Config.Session.TimeOut)

		// 然后 把 消息 发给 openai
		go func() {
			// 去通过 embeddings 进行数据匹配
			type EmbeddingData struct {
				Q string `json:"q"`
				A string `json:"a"`
			}
			var embeddingData []EmbeddingData
			// 为了避免 embedding 的冷启动问题，对问题进行缓存来避免冷启动, 先简单处理
			matchEmbeddings := len(l.svcCtx.Config.Embeddings.Mlvus.Keywords) == 0
			for _, keyword := range l.svcCtx.Config.Embeddings.Mlvus.Keywords {
				if strings.Contains(req.Msg, keyword) {
					matchEmbeddings = true
				}
			}

			if l.svcCtx.Config.Embeddings.Enable && matchEmbeddings {
				// md5 this req.MSG to key
				key := md5.New()
				_, _ = io.WriteString(key, req.Msg)
				keyStr := fmt.Sprintf("%x", key.Sum(nil))
				type EmbeddingCache struct {
					Embedding []float64 `json:"embedding"`
				}
				embeddingRes, err := redis.Rdb.Get(context.Background(), fmt.Sprintf(redis.EmbeddingsCacheKey, keyStr)).Result()
				if err == nil {
					tmp := new(EmbeddingCache)
					_ = json.Unmarshal([]byte(embeddingRes), tmp)

					result := milvus.Search(tmp.Embedding, l.svcCtx.Config.Embeddings.Mlvus.Host)
					tempMessage := ""
					for _, qa := range result {
						if qa.Score > 0.3 {
							continue
						}
						if len(embeddingData) < 2 {
							embeddingData = append(embeddingData, EmbeddingData{
								Q: qa.Q,
								A: qa.A,
							})
						} else {
							tempMessage += qa.Q + "\n"
						}
					}
					if tempMessage != "" {
						go sendToUser(req.OpenKfID, req.CustomerID, "正在思考中，也许您还想知道"+"\n\n"+tempMessage, l.svcCtx.Config)
					}
				} else {
					go sendToUser(req.OpenKfID, req.CustomerID, "正在为您搜索相关数据", l.svcCtx.Config)
					res, err := c.CreateOpenAIEmbeddings(req.Msg)
					if err == nil {
						embedding := res.Data[0].Embedding
						// 去将其存入 redis
						embeddingCache := EmbeddingCache{
							Embedding: embedding,
						}
						redisData, err := json.Marshal(embeddingCache)
						if err == nil {
							redis.Rdb.Set(context.Background(), fmt.Sprintf(redis.EmbeddingsCacheKey, keyStr), string(redisData), -1*time.Second)
						}
						// 将 embedding 数据与 milvus 数据库 内的数据做对比响应前3个相关联的数据
						result := milvus.Search(embedding, l.svcCtx.Config.Embeddings.Mlvus.Host)

						tempMessage := ""
						for _, qa := range result {
							if qa.Score > 0.3 {
								continue
							}
							if len(embeddingData) < 2 {
								embeddingData = append(embeddingData, EmbeddingData{
									Q: qa.Q,
									A: qa.A,
								})
							} else {
								tempMessage += qa.Q + "\n"
							}
						}
						if tempMessage != "" {
							go sendToUser(req.OpenKfID, req.CustomerID, "正在思考中，也许您还想知道"+"\n\n"+tempMessage, l.svcCtx.Config)
						}
					}
				}
			}

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
				pc := c
				pluginInfo, err := pc.WithMaxToken(1000).WithTemperature(0).
					Chat(plugin.GetChatPluginPromptInfo(req.Msg, p))
				if err == nil {
					runPluginInfo, ok := plugin.RunPlugin(pluginInfo, p)
					if ok {
						if runPluginInfo.Wrapper == false {
							// 插件处理成功，发送给用户
							go sendToUser(req.OpenKfID, req.CustomerID, runPluginInfo.Output, l.svcCtx.Config)
							return
						}
						q := fmt.Sprintf(
							"根据用户输入\n%s\n\nai决定使用%s插件\nai请求插件的信息为: %s\n通过插件获取到的响应信息为: %s\n 。请确认以上信息，如果信息中存在与你目前信息不一致的地方，请以上方%s插件提供的信息为准，比如日期... 并将其作为后续回答的依据，确认请回复 ok ,不要解释",
							req.Msg, runPluginInfo.PluginName, runPluginInfo.Input, runPluginInfo.Output, runPluginInfo.PluginName,
						)
						// 插件处理成功，存入上下文
						collection.Set(openai.NewChatContent(q), "ok", conversationId, false)
						// 客服消息不开启 debug 模式，因为响应条数 5条的限制
					}
				}
			}

			// 基于 summary 进行补充
			messageText := ""
			for _, chat := range embeddingData {
				collection.Set(openai.NewChatContent(chat.Q), chat.A, conversationId, false)
			}
			collection.Set(openai.NewChatContent(req.Msg), "", conversationId, false)

			prompts := collection.GetChatSummary()
			if l.svcCtx.Config.Response.Stream {
				channel := make(chan string, 100)
				go func() {
					if l.model == openai.TextModel {
						messageText, err = c.CompletionStream(prompts, channel)
					} else {
						messageText, err = c.ChatStream(prompts, channel)
					}
					if err != nil {
						logx.Error("读取 stream 失败：", err.Error())
						sendToUser(req.OpenKfID, req.CustomerID, "系统拥挤，稍后再试~"+err.Error(), l.svcCtx.Config)
						return
					}
					collection.Set(openai.NewChatContent(), messageText, conversationId, true)
					// 再去插入数据
					table := l.svcCtx.ChatModel.Chat
					_ = table.WithContext(context.Background()).Create(&model.Chat{
						User:       req.CustomerID,
						OpenKfID:   req.OpenKfID,
						MessageID:  req.MsgID,
						ReqContent: req.Msg,
						ResContent: messageText,
					})
				}()

				var rs []rune
				// 加快初次响应的时间 后续可改为阶梯式（用户体验好）
				first := true
				for {
					s, ok := <-channel
					if !ok {
						// 数据接受完成
						if len(rs) > 0 {
							go sendToUser(req.OpenKfID, req.CustomerID,
								string(rs)+"\n--------------------------------\n"+req.Msg,
								l.svcCtx.Config,
							)
						}
						return
					}
					rs = append(rs, []rune(s)...)

					if first && len(rs) > 50 && strings.Contains(s, "\n\n") {
						go sendToUser(req.OpenKfID, req.CustomerID, strings.TrimRight(string(rs), "\n\n"), l.svcCtx.Config)
						rs = []rune{}
						first = false
					} else if len(rs) > 200 && strings.Contains(s, "\n\n") {
						go sendToUser(req.OpenKfID, req.CustomerID, strings.TrimRight(string(rs), "\n\n"), l.svcCtx.Config)
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
				sendToUser(req.OpenKfID, req.CustomerID, "系统错误:"+err.Error(), l.svcCtx.Config)
				return
			}

			// 然后把数据 发给对应的客户
			go sendToUser(req.OpenKfID, req.CustomerID, messageText, l.svcCtx.Config)
			collection.Set(openai.NewChatContent(), messageText, conversationId, true)
			table := l.svcCtx.ChatModel.Chat
			_ = table.WithContext(context.Background()).Create(&model.Chat{
				User:       req.CustomerID,
				OpenKfID:   req.OpenKfID,
				MessageID:  req.MsgID,
				ReqContent: req.Msg,
				ResContent: messageText,
			})
		}()
	}
	return &types.CustomerChatReply{
		Message: "ok",
	}, nil
}

func (l *CustomerChatLogic) setModelName() (ls *CustomerChatLogic) {
	m := openai.ChatModel
	for _, s := range l.svcCtx.Config.WeCom.MultipleApplication {
		if s.ManageAllKFSession {
			m = s.Model
		}
	}
	m = strings.ToLower(m)
	if _, ok := openai.Models[m]; !ok {
		m = openai.ChatModel
	}
	l.model = m
	return l
}

func (l *CustomerChatLogic) setBasePrompt() (ls *CustomerChatLogic) {
	p := ""
	for _, s := range l.svcCtx.Config.WeCom.MultipleApplication {
		if s.ManageAllKFSession {
			p = s.BasePrompt
		}
	}
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

func (l *CustomerChatLogic) FactoryCommend(req *types.CustomerChatReq) (proceed bool, err error) {
	template := make(map[string]CustomerTemplateData)
	//当 message 以 # 开头时，表示是特殊指令
	if !strings.HasPrefix(req.Msg, "#") {
		return true, nil
	}

	template["#direct"] = CustomerCommendDirect{}
	//template["#voice"] = CustomerCommendVoice{}
	template["#help"] = CustomerCommendHelp{}
	template["#system"] = CustomerCommendSystem{}
	template["#clear"] = CustomerCommendClear{}
	template["#about"] = CustomerCommendAbout{}
	template["#plugin"] = CustomerPlugin{}
	template["#image"] = CustomerCommendImage{}

	for s, data := range template {
		if strings.HasPrefix(req.Msg, s) {
			proceed = data.customerExec(l, req)
			return proceed, nil
		}
	}

	return true, nil
}

type CustomerTemplateData interface {
	customerExec(svcCtx *CustomerChatLogic, req *types.CustomerChatReq) (proceed bool)
}

type CustomerCommendVoice struct{}

func (p CustomerCommendVoice) customerExec(l *CustomerChatLogic, req *types.CustomerChatReq) bool {
	msg := strings.Replace(req.Msg, "#voice:", "", -1)
	if msg == "" {
		sendToUser(req.OpenKfID, req.CustomerID, "系统错误:未能读取到音频信息", l.svcCtx.Config)
		return false
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
	case "ali":
		logx.Info("使用阿里云音频转换")
		//s, err := voice.NewSpeakClient(
		//	l.svcCtx.Config.Speaker.AliYun.AccessKeyId,
		//	l.svcCtx.Config.Speaker.AliYun.AccessKeySecret,
		//	l.svcCtx.Config.Speaker.AliYun.AppKey,
		//)
		//if err != nil {
		//	wecom.SendCustomerChatMessage(req.OpenKfID, req.CustomerID, "阿里云音频转换初始化失败:"+err.Error())
		//	return false
		//}
		//msg = strings.Replace(msg, ".mp3", ".amr", -1)
		//cli = s
	default:
		sendToUser(req.OpenKfID, req.CustomerID, "系统错误:未知的音频转换服务商", l.svcCtx.Config)
		return false
	}

	txt, err := cli.SpeakToTxt(msg)
	if txt == "" || err != nil {
		logx.Info("openai转换错误", err.Error())
		sendToUser(req.OpenKfID, req.CustomerID, "系统错误:音频信息转换错误", l.svcCtx.Config)
		return false
	}
	// 语音识别成功
	sendToUser(req.OpenKfID, req.CustomerID, "语音识别成功:\n\n"+txt+"\n\n系统正在思考中...", l.svcCtx.Config)
	l.message = txt
	return true
}

type CustomerCommendClear struct{}

func (p CustomerCommendClear) customerExec(l *CustomerChatLogic, req *types.CustomerChatReq) bool {
	// 清理上下文
	openai.NewUserContext(
		openai.GetUserUniqueID(req.CustomerID, req.OpenKfID),
	).Clear()
	sendToUser(req.OpenKfID, req.CustomerID, "记忆清除完成:来开始新一轮的chat吧", l.svcCtx.Config)
	return false
}

// CustomerCommendSystem 查看系统信息
type CustomerCommendSystem struct{}

func (p CustomerCommendSystem) customerExec(l *CustomerChatLogic, req *types.CustomerChatReq) bool {
	tips := fmt.Sprintf(
		"系统信息\n系统版本为：%s \nmodel 版本为：%s \n系统基础设定：%s \n",
		l.svcCtx.Config.SystemVersion,
		l.model,
		l.basePrompt,
	)
	sendToUser(req.OpenKfID, req.CustomerID, tips, l.svcCtx.Config)
	return false
}

// CustomerCommendHelp 查看所有指令
type CustomerCommendHelp struct{}

func (p CustomerCommendHelp) customerExec(l *CustomerChatLogic, req *types.CustomerChatReq) bool {
	tips := fmt.Sprintf(
		"支持指令：\n\n%s\n%s\n%s\n%s\n%s\n",
		"基础模块🕹️\n\n#help       查看所有指令",
		"#system 查看会话系统信息",
		"#clear 清空当前会话的数据",
		"\n插件🛒\n",
		"#plugin:list 查看所有插件",
	)
	sendToUser(req.OpenKfID, req.CustomerID, tips, l.svcCtx.Config)
	return false
}

type CustomerCommendAbout struct{}

func (p CustomerCommendAbout) customerExec(l *CustomerChatLogic, req *types.CustomerChatReq) bool {
	sendToUser(req.OpenKfID, req.CustomerID, "https://github.com/whyiyhw/chatgpt-wechat", l.svcCtx.Config)
	return false
}

type CustomerCommendDirect struct{}

func (p CustomerCommendDirect) customerExec(l *CustomerChatLogic, req *types.CustomerChatReq) bool {
	msg := strings.Replace(req.Msg, "#direct:", "", -1)
	sendToUser(req.OpenKfID, req.CustomerID, msg, l.svcCtx.Config)
	return false
}

type CustomerPlugin struct{}

func (p CustomerPlugin) customerExec(l *CustomerChatLogic, req *types.CustomerChatReq) bool {
	if strings.HasPrefix(req.Msg, "#plugin") {
		if strings.HasPrefix(req.Msg, "#plugin:list") {
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
			sendToUser(req.OpenKfID, req.CustomerID, fmt.Sprintf("当前可用的插件列表：\n%s", pluginStr), l.svcCtx.Config)
			return false
		}
	}
	return true
}

type CustomerCommendImage struct{}

func (p CustomerCommendImage) customerExec(l *CustomerChatLogic, req *types.CustomerChatReq) bool {
	// #image:https://www.baidu.com/img/bd_logo1.png
	msg := strings.Replace(req.Msg, "#image:", "", -1)
	if msg == "" {
		sendToUser(req.OpenKfID, req.CustomerID, "请输入完整的设置 如：#image:https://www.google.com/img/bd_logo1.png", l.svcCtx.Config)
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
		sendToUser(req.OpenKfID, req.CustomerID, "图片解析失败:"+err.Error(), l.svcCtx.Config)
		return false
	}
	sendToUser(req.OpenKfID, req.CustomerID, "好的收到了您的图片，正在识别中~", l.svcCtx.Config)
	result, err := c.Chat(append(parseImage, gemini.ChatModelMessage{
		Role:    gemini.UserRole,
		Content: gemini.NewChatContent(base64Data, mime),
	}, gemini.ChatModelMessage{
		Role:    gemini.UserRole,
		Content: gemini.NewChatContent("你能详细描述图片中的内容吗？"),
	}))
	if err != nil {
		sendToUser(req.OpenKfID, req.CustomerID, "图片识别失败:"+err.Error(), l.svcCtx.Config)
		return false
	}

	sendToUser(req.OpenKfID, req.CustomerID, "图片识别完成:\n\n"+result, l.svcCtx.Config)
	// 将其存入 上下文
	gemini.NewUserContext(
		openai.GetUserUniqueID(req.CustomerID, req.OpenKfID),
	).WithModel(c.Model).
		WithPrompt(l.svcCtx.Config.Gemini.Prompt).
		WithClient(c).
		Set(
			gemini.NewChatContent("我向你描述一副图片的内容如下：\n\n"+result),
			"收到,我了解了您的图片！",
			"",
			true,
		)
	return false
}
