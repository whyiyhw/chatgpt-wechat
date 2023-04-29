package logic

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"chat/common/ali/ocr"
	"chat/common/milvus"
	"chat/common/openai"
	"chat/common/plugin"
	"chat/common/redis"
	"chat/common/wecom"
	"chat/service/chat/api/internal/config"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type ChatLogic struct {
	logx.Logger
	ctx        context.Context
	svcCtx     *svc.ServiceContext
	model      string
	baseHost   string
	basePrompt string
	message    string
}

func NewChatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatLogic {
	return &ChatLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChatLogic) Chat(req *types.ChatReq) (resp *types.ChatReply, err error) {

	// 去找 openai 获取数据
	if req.Channel == "openai" {
		l.setModelName(req.AgentID).setBasePrompt(req.AgentID).setBaseHost()

		// 如果用户有自定义的配置，就使用用户的配置
		configCollection, configErr := l.svcCtx.ChatConfigModel.FindOneByQuery(
			context.Background(),
			l.svcCtx.ChatConfigModel.RowBuilder().
				Where(squirrel.Eq{"user": req.UserID}).
				Where(squirrel.Eq{"agent_id": req.AgentID}).
				OrderBy("id desc"),
		)
		if configErr == nil && configCollection.Id > 0 {
			l.basePrompt = configCollection.Prompt
			l.model = configCollection.Model
		}

		// 指令匹配， 根据响应值判定是否需要去调用 openai 接口了
		proceed, _ := l.FactoryCommend(req)
		if !proceed {
			return
		}
		if l.message != "" {
			req.MSG = l.message
		}

		// openai client
		c := openai.NewChatClient(l.svcCtx.Config.OpenAi.Key).
			WithModel(l.model).
			WithBaseHost(l.baseHost).
			WithOrigin(l.svcCtx.Config.OpenAi.Origin).
			WithEngine(l.svcCtx.Config.OpenAi.Engine)
		if l.svcCtx.Config.Proxy.Enable {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5)
		}

		// context
		collection := openai.NewUserContext(
			openai.GetUserUniqueID(req.UserID, strconv.FormatInt(req.AgentID, 10)),
		).WithModel(l.model).WithPrompt(l.basePrompt).WithClient(c)

		go func() {
			// 去通过 embeddings 进行数据匹配
			type EmbeddingData struct {
				Q string `json:"q"`
				A string `json:"a"`
			}
			var embeddingData []EmbeddingData
			// 为了避免 embedding 的冷启动问题，对问题进行缓存来避免冷启动, 先简单处理
			if l.svcCtx.Config.Embeddings.Enable {
				matchEmbeddings := len(l.svcCtx.Config.Embeddings.Mlvus.Keywords) == 0
				for _, keyword := range l.svcCtx.Config.Embeddings.Mlvus.Keywords {
					if strings.Contains(req.MSG, keyword) {
						matchEmbeddings = true
					}
				}
				if matchEmbeddings {
					// md5 this req.MSG to key
					key := md5.New()
					_, _ = io.WriteString(key, req.MSG)
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
							go sendToUser(req.AgentID, req.UserID, "正在思考中，也许您还想知道"+"\n\n"+tempMessage, l.svcCtx.Config)
						}
					} else {
						sendToUser(req.AgentID, req.UserID, "正在为您查询相关数据", l.svcCtx.Config)
						res, err := c.CreateOpenAIEmbeddings(req.MSG)
						if err == nil {
							fmt.Println(res.Data)
							fmt.Println(l.svcCtx.Config.Embeddings)
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
								go sendToUser(req.AgentID, req.UserID, "正在思考中，也许您还想知道"+"\n\n"+tempMessage, l.svcCtx.Config)
							}
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
				pluginInfo, err := c.WithMaxToken(1000).WithTemperature(0).
					Completion(plugin.GetPluginPromptInfo(req.MSG, p))
				if err == nil {
					msg, ok := plugin.RunPlugin(pluginInfo, p)
					if ok && msg != "" {
						// 插件处理成功，存入上下文
						collection.Set(req.MSG+"\n"+msg+
							"\n 你已经确认以上信息，如果信息中存在与你目前信息不一致的地方，请以上方插件提供的信息为准，比如日期... 并将其作为后续回答的依据，确认请回复 ok",
							"ok", false,
						)
					}
				}
			}

			// 基于 summary 进行补充
			messageText := ""
			for _, chat := range embeddingData {
				collection.Set(chat.Q, chat.A, false)
			}
			collection.Set(req.MSG, "", false)

			if l.model == openai.TextModel {
				messageText, err = c.Completion(collection.GetCompletionSummary())
				collection.Set("", messageText, true)
			} else {
				prompts := collection.GetChatSummary()

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
						collection.Set("", messageText, true)
						// 再去插入数据
						_, _ = l.svcCtx.ChatModel.Insert(context.Background(), &model.Chat{
							AgentId:    req.AgentID,
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

			collection.Set("", messageText, true)
			// 再去插入数据
			_, _ = l.svcCtx.ChatModel.Insert(context.Background(), &model.Chat{
				AgentId:    req.AgentID,
				User:       req.UserID,
				ReqContent: req.MSG,
				ResContent: messageText,
			})
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
	m := l.svcCtx.Config.WeCom.Model
	for _, application := range l.svcCtx.Config.WeCom.MultipleApplication {
		if application.AgentID == agentID {
			m = application.Model
		}
	}
	if m == "" || (m != openai.TextModel && m != openai.ChatModel && m != openai.ChatModelNew && m != openai.ChatModel4) {
		m = openai.TextModel
	}
	l.model = m
	return l
}

func (l *ChatLogic) setBasePrompt(agentID int64) (ls *ChatLogic) {
	p := l.svcCtx.Config.WeCom.BasePrompt
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

	for s, data := range template {
		if strings.HasPrefix(req.MSG, s) {
			proceed = data.exec(l, req)
			return proceed, nil
		}
	}

	return true, nil
}

func sendToUser(agentID int64, userID, msg string, config config.Config, images ...string) {
	// 确认多应用模式是否开启
	corpSecret := config.WeCom.DefaultAgentSecret
	// 兼容性调整 取 DefaultAgentSecret 作为默认值 兼容老版本 CorpSecret
	if corpSecret == "" {
		corpSecret = config.WeCom.CorpSecret
	}
	for _, application := range config.WeCom.MultipleApplication {
		if application.AgentID == agentID {
			corpSecret = application.AgentSecret
		}
	}
	wecom.SendToWeComUser(agentID, userID, msg, corpSecret, images...)
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
	sendToUser(req.AgentID, req.UserID, "当前会话清理完成，来开始新一轮的chat吧", l.svcCtx.Config)
	return false
}

// CommendHelp 查看所有指令
type CommendHelp struct{}

func (p CommendHelp) exec(l *ChatLogic, req *types.ChatReq) bool {
	tips := fmt.Sprintf(
		"支持指令：\n\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n",
		"基础模块🕹️\n\n#help 查看所有指令",
		"#system 查看当前对话的系统信息",
		"#clear 清空当前会话的数据\n",
		"会话设置🦄\n\n#config_prompt:xxx，如程序员的小助手",
		"#config_model:xxx，如text-davinci-003",
		"#config_clear 初始化对话设置",
		"#prompt:list 查看所有支持的预定义角色",
		"#prompt:set:xx 如 24(诗人)，角色应用",
		"\n会话控制🚀\n",
		"#session:start 开启新的会话",
		"#session:list  查看所有会话",
		"#session:clear 清空所有会话",
		"#session:exchange:xxx 切换指定会话",
		"\n绘图🎨\n",
		"#draw:xxx 按照指定 prompt 进行绘画",
	)
	sendToUser(req.AgentID, req.UserID, tips, l.svcCtx.Config)
	return false
}

type CommendSystem struct{}

func (p CommendSystem) exec(l *ChatLogic, req *types.ChatReq) bool {
	tips := "系统信息\n model 版本为：" + l.model + "\n 系统基础设定：" + l.basePrompt + " \n"
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
		AgentId: req.AgentID,
		User:    req.UserID,
		Prompt:  msg,
		Model:   l.model,
	}
	_, configErr := l.svcCtx.ChatConfigModel.Insert(context.Background(), &chatConfig)

	if configErr != nil {
		sendToUser(req.AgentID, req.UserID, "设置失败,请稍后再试~", l.svcCtx.Config)
		return false
	}

	sendToUser(req.AgentID, req.UserID, "设置成功，您目前的对话配置如下：\n prompt: "+msg+"\n model: "+l.model, l.svcCtx.Config)
	return false
}

type CommendConfigModel struct{}

func (p CommendConfigModel) exec(l *ChatLogic, req *types.ChatReq) bool {
	// #config_model:您的设置 如 text-davinci-003\n
	msg := strings.Trim(strings.Replace(req.MSG, "#config_model:", "", -1), " ")

	if msg == "" {
		sendToUser(req.AgentID, req.UserID, "请输入完整的设置 如：\n#config_model:text-davinci-003", l.svcCtx.Config)
		return false
	}

	if msg != openai.TextModel && msg != openai.ChatModel && msg != openai.ChatModelNew && msg != openai.ChatModel4 {
		tips := fmt.Sprintf("目前只支持以下四种模型：\n %s \n %s \n %s \n %s \n", openai.TextModel, openai.ChatModel, openai.ChatModelNew, openai.ChatModel4)
		sendToUser(req.AgentID, req.UserID, tips, l.svcCtx.Config)
		return false
	}

	// 去数据库新增用户的对话配置
	chatConfig := model.ChatConfig{
		AgentId: req.AgentID,
		User:    req.UserID,
		Prompt:  l.basePrompt,
		Model:   msg,
	}
	_, configErr := l.svcCtx.ChatConfigModel.Insert(context.Background(), &chatConfig)

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
	builder := l.svcCtx.ChatConfigModel.RowBuilder().Where(squirrel.Eq{"user": req.UserID}).Where(squirrel.Eq{"agent_id": req.AgentID})
	collection, _ := l.svcCtx.ChatConfigModel.FindAll(context.Background(), builder)
	for _, val := range collection {
		_ = l.svcCtx.ChatConfigModel.Delete(context.Background(), val.Id)
	}
	sendToUser(req.AgentID, req.UserID, "对话设置已恢复初始化", l.svcCtx.Config)
	return false
}

type CommendWelcome struct{}

func (p CommendWelcome) exec(l *ChatLogic, req *types.ChatReq) bool {
	cacheKey := fmt.Sprintf(redis.WelcomeCacheKey, req.AgentID, req.UserID)
	if _, err := redis.Rdb.Get(context.Background(), cacheKey).Result(); err == nil {
		return false
	}
	sendToUser(req.AgentID, req.UserID, l.svcCtx.Config.WeCom.Welcome, l.svcCtx.Config)
	_, err := redis.Rdb.SetEX(context.Background(), cacheKey, "1", 24*15*time.Hour).Result()
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
	vi := reflect.ValueOf(l.svcCtx.Config.OCR)
	if vi.Kind() == reflect.Ptr && vi.IsNil() {
		sendToUser(req.AgentID, req.UserID, "请先配置OCR", l.svcCtx.Config)
		return false
	}
	if l.svcCtx.Config.OCR.Company != "ali" {
		sendToUser(req.AgentID, req.UserID, "目前只支持阿里OCR", l.svcCtx.Config)
		return false
	}
	ocrCli, _err := ocr.CreateClient(&l.svcCtx.Config.OCR.AliYun.AccessKeyId, &l.svcCtx.Config.OCR.AliYun.AccessKeySecret)
	if _err != nil {
		// 创建失败
		sendToUser(req.AgentID, req.UserID, "图片识别客户端创建失败失败:"+_err.Error(), l.svcCtx.Config)
		return false
	}

	// 进行图片识别
	txt, err := ocr.Image2Txt(msg, ocrCli)
	if err != nil {
		sendToUser(req.AgentID, req.UserID, "图片识别失败:"+err.Error(), l.svcCtx.Config)
		return false
	}
	if msg == "" {
		sendToUser(req.AgentID, req.UserID, "图片识别失败:"+err.Error(), l.svcCtx.Config)
		return false
	}
	// 图片识别成功
	sendToUser(req.AgentID, req.UserID, "图片识别成功:\n\n"+txt, l.svcCtx.Config)

	l.message = txt
	return true
}

type CommendPromptList struct{}

func (p CommendPromptList) exec(l *ChatLogic, req *types.ChatReq) bool {
	// #prompt:list
	// 去数据库获取用户的所有prompt
	collection, _ := l.svcCtx.PromptConfigModel.FindAll(context.Background(),
		l.svcCtx.PromptConfigModel.RowBuilder().Where(squirrel.Gt{"id": 1}),
	)
	var prompts []string
	for _, val := range collection {
		prompts = append(prompts, fmt.Sprintf("%s:%d", val.Key, val.Id))
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
	prompt, _err := l.svcCtx.PromptConfigModel.FindOne(context.Background(), mId)
	switch _err {
	case model.ErrNotFound:
		sendToUser(req.AgentID, req.UserID, "此 prompt 不存在，请确认后再试", l.svcCtx.Config)
	case nil:
		// 去数据库新增用户的对话配置
		chatConfig := model.ChatConfig{
			AgentId: req.AgentID,
			User:    req.UserID,
			Prompt:  prompt.Value,
			Model:   l.model,
		}
		_, configErr := l.svcCtx.ChatConfigModel.Insert(context.Background(), &chatConfig)

		if configErr != nil {
			sendToUser(req.AgentID, req.UserID, msg+"设置失败:"+configErr.Error(), l.svcCtx.Config)
			return false
		}
		sendToUser(req.AgentID, req.UserID, "设置成功，您目前的对话配置如下：\n prompt: "+prompt.Value+"\n model: "+l.model, l.svcCtx.Config)
	default:
		sendToUser(req.AgentID, req.UserID, "设置失败, prompt 查询失败"+err.Error(), l.svcCtx.Config)
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

	c := openai.NewChatClient(l.svcCtx.Config.OpenAi.Key).
		WithModel(l.model).
		WithBaseHost(l.svcCtx.Config.OpenAi.Host).
		WithOrigin(l.svcCtx.Config.OpenAi.Origin).
		WithEngine(l.svcCtx.Config.OpenAi.Engine)

	if l.svcCtx.Config.Proxy.Enable {
		c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5)
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
		//logx.Info("使用阿里云音频转换")
		//s, err := voice.NewSpeakClient(
		//	l.svcCtx.Config.Speaker.AliYun.AccessKeyId,
		//	l.svcCtx.Config.Speaker.AliYun.AccessKeySecret,
		//	l.svcCtx.Config.Speaker.AliYun.AppKey,
		//)
		//if err != nil {
		//	sendToUser(req.AgentID, req.UserID, "阿里云音频转换初始化失败:"+err.Error(), l.svcCtx.Config)
		//	return false
		//}
		//msg = strings.Replace(msg, ".mp3", ".amr", -1)
		//cli = s
	default:
		sendToUser(req.AgentID, req.UserID, "系统错误:未知的音频转换服务商", l.svcCtx.Config)
		return false
	}
	fmt.Println(cli)
	txt, err := cli.SpeakToTxt(msg)
	if txt == "" {
		sendToUser(req.AgentID, req.UserID, "音频信息转换错误:"+err.Error(), l.svcCtx.Config)
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

	sendToUser(req.AgentID, req.UserID, "未知的命令，您可以通过 \n#help \n查看帮助", l.svcCtx.Config)
	return false
}

type CommendDraw struct{}

func (p CommendDraw) exec(l *ChatLogic, req *types.ChatReq) bool {
	if strings.HasPrefix(req.MSG, "#draw:") {
		prompt := strings.Replace(req.MSG, "#draw:", "", -1)
		if l.svcCtx.Config.Draw.Enable {
			host := l.svcCtx.Config.Draw.StableDiffusion.Host
			url := host + "/sdapi/v1/txt2img"
			reqPayload := map[string]interface{}{
				"prompt": prompt,
				"steps":  20,
			}
			tokenStr := l.svcCtx.Config.Draw.StableDiffusion.Auth.Username + ":" + l.svcCtx.Config.Draw.StableDiffusion.Auth.Password
			encodedToken := base64.StdEncoding.EncodeToString([]byte(tokenStr))

			client := &http.Client{}
			body, _ := json.Marshal(reqPayload)
			drawReq, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
			if err != nil {
				logx.Info("draw request client build fail", err)
				sendToUser(req.AgentID, req.UserID, "构建绘画请求失败，请重新尝试~", l.svcCtx.Config)
				return false
			}
			logx.Info("draw request client build success")
			drawReq.Header.Add("Content-Type", "application/json")
			drawReq.Header.Add("Authorization", "Basic "+encodedToken)

			sendToUser(req.AgentID, req.UserID, "正在绘画中...", l.svcCtx.Config)

			res, err := client.Do(drawReq)
			if err != nil {
				logx.Info("draw request fail", err)
				sendToUser(req.AgentID, req.UserID, "绘画请求失败，请重新尝试~", l.svcCtx.Config)
				return false
			}
			defer func(Body io.ReadCloser) {
				_ = Body.Close()
			}(res.Body)

			resBody, err := io.ReadAll(res.Body)
			if err != nil {
				logx.Info("draw request fail", err)
				sendToUser(req.AgentID, req.UserID, "绘画请求响应失败，请重新尝试~", l.svcCtx.Config)
				return false
			}

			var resPayload map[string]interface{}
			err = json.Unmarshal(resBody, &resPayload)
			if err != nil {
				logx.Info("draw request fail", err)
				sendToUser(req.AgentID, req.UserID, "绘画请求响应解析失败，请重新尝试~", l.svcCtx.Config)
				return false
			}
			images := resPayload["images"].([]interface{})
			for _, image := range images {
				s := image.(string)
				if err != nil {
					logx.Info("draw request fail", err)
					sendToUser(req.AgentID, req.UserID, "绘画请求响应解析失败，请重新尝试~", l.svcCtx.Config)
					return false
				}
				// 将解密后的信息写入到本地
				imageBase64 := strings.Split(s, ",")[0]
				decodeBytes, err := base64.StdEncoding.DecodeString(imageBase64)
				if err != nil {
					logx.Info("draw request fail", err)
					sendToUser(req.AgentID, req.UserID, "绘画请求响应解析失败，请重新尝试~", l.svcCtx.Config)
					return false
				}

				// 判断目录是否存在
				_, err = os.Stat("/tmp/image")
				if err != nil {
					err := os.MkdirAll("/tmp/image", os.ModePerm)
					if err != nil {
						fmt.Println("mkdir err:", err)
						sendToUser(req.AgentID, req.UserID, "绘画请求响应解析失败，请重新尝试~", l.svcCtx.Config)
						return false
					}
				}

				path := fmt.Sprintf("/tmp/image/%s.png", uuid.New().String())

				err = os.WriteFile(path, decodeBytes, os.ModePerm)

				if err != nil {
					logx.Info("draw save fail", err)
					sendToUser(req.AgentID, req.UserID, "绘画请求响应解析失败，请重新尝试~", l.svcCtx.Config)
					return false
				}

				// 再将 image 信息发送到用户
				sendToUser(req.AgentID, req.UserID, "", l.svcCtx.Config, path)
				return false
			}
		}
	}
	sendToUser(req.AgentID, req.UserID, "未知的命令，您可以通过 \n#help \n查看帮助", l.svcCtx.Config)
	return false
}
