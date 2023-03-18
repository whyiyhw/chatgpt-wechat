package logic

import (
	"chat/common/aliocr"
	"chat/common/openai"
	"chat/common/wecom"
	"chat/service/chat/api/internal/config"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"io"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/net/proxy"
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
		url := "https://api.openai.com/v1/completions"

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

		go func() {
			// 从数据库中读取 用户的所有 请求与 响应数据 进行请求拼接
			whereBuilder := l.svcCtx.ChatModel.RowBuilder().Where(squirrel.Eq{"user": req.UserID}).Where(squirrel.Eq{"agent_id": req.AgentID})
			collection, _ := l.svcCtx.ChatModel.FindAll(context.Background(), whereBuilder)

			var bytes []byte
			skipNum := 0
			if len(collection) > 20 {
				skipNum = len(collection) - 20
			}
			if l.model == openai.TextModel {
				// TODO 将对话进行总结 然后拿总结的话进行构造请求与回复
				for k, val := range collection {
					if k >= skipNum {
						l.basePrompt += "Q: " + val.ReqContent + "\nA: " + val.ResContent + "<|im_end|>\n"
					}
				}
				l.basePrompt += "\nQ: " + req.MSG + "\nA: "

				bytes = openai.TextModelRequestBuild(l.basePrompt)

				url = l.baseHost + "/v1/completions"
			} else {
				var prompts []openai.ChatModelMessage
				prompts = append(prompts, openai.ChatModelMessage{
					Role:    "system",
					Content: l.basePrompt,
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
				url = l.baseHost + "/v1/chat/completions"
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
							sendToUser(req.AgentID, req.UserID, "上下文超过最大长度的限制，请输入 #clear 来清理上下文", l.svcCtx.Config)
							return
						}
					}
				}

				messageText = fmt.Sprintf("%s \n\n系统错误，请清理后重试: type:%v code:%v",
					req.MSG, openAiResError.Error.Type, openAiResError.Error.Code,
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

func (l *ChatLogic) setBaseHost() (ls *ChatLogic) {
	if l.svcCtx.Config.OpenAi.Host == "" {
		l.svcCtx.Config.OpenAi.Host = "https://api.openai.com"
	}
	l.baseHost = l.svcCtx.Config.OpenAi.Host
	return l
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
	l.svcCtx.Config.WeCom.Model = m
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
	template["#config_prompt:"] = CommendConfigPrompt{}
	template["#config_model:"] = CommendConfigModel{}
	template["#config_clear"] = CommendConfigClear{}
	template["#help"] = CommendHelp{}
	template["#image"] = CommendImage{}
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

type TemplateData interface {
	exec(svcCtx *ChatLogic, req *types.ChatReq) (proceed bool)
}

// CommendClear 清除用户的所有对话数据
type CommendClear struct{}

func (p CommendClear) exec(l *ChatLogic, req *types.ChatReq) bool {
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
	return false
}

// CommendHelp 查看所有指令
type CommendHelp struct{}

func (p CommendHelp) exec(l *ChatLogic, req *types.ChatReq) bool {
	tips := fmt.Sprintf(
		"目前支持的指令有：\n %s\n %s\n %s\n %s\n %s\n %s\n %s\n %s",
		"#clear 清空当前应用的对话数据",
		"#help 查看所有指令",
		"#config_prompt:您的设置，如程序员的小助手",
		"#config_model:您的设置，如text-davinci-003",
		"#config_clear:设置当前应用的对话设置至初始值，并清理会话记录",
		"#prompt:list 查看所有支持的预定义角色",
		"#prompt:set:您的设置，如 24 (诗人)",
		"#system 查看当前对话的系统信息",
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
	// 去数据库删除 用户的所有对话数据
	chatBuilder := l.svcCtx.ChatModel.RowBuilder().
		Where(squirrel.Eq{"user": req.UserID}).
		Where(squirrel.Eq{"agent_id": req.AgentID})
	chatCollection, _ := l.svcCtx.ChatModel.FindAll(context.Background(), chatBuilder)
	for _, val := range chatCollection {
		_ = l.svcCtx.ChatModel.Delete(context.Background(), val.Id)
	}
	sendToUser(req.AgentID, req.UserID, "对话配置,与会话设置清除完成", l.svcCtx.Config)
	return false
}

type CommendWelcome struct{}

func (p CommendWelcome) exec(l *ChatLogic, req *types.ChatReq) bool {
	cacheKey := fmt.Sprintf("chat:wecome:%d:%s", req.AgentID, req.UserID)
	rdb := redis.NewClient(&redis.Options{
		Addr:     l.svcCtx.Config.RedisCache[0].Host,
		Password: l.svcCtx.Config.RedisCache[0].Pass,
		DB:       1,
	})
	defer func(rdb *redis.Client) {
		err := rdb.Close()
		if err != nil {
			fmt.Println("welcome3:" + err.Error())
		}
	}(rdb)
	if _, err := rdb.Get(context.Background(), cacheKey).Result(); err == nil {
		return false
	}
	sendToUser(req.AgentID, req.UserID, l.svcCtx.Config.WeCom.Welcome, l.svcCtx.Config)
	_, err := rdb.SetEX(context.Background(), cacheKey, "1", 24*15*time.Hour).Result()
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
	ocrCli, _err := aliocr.CreateClient(&l.svcCtx.Config.OCR.AliYun.AccessKeyId, &l.svcCtx.Config.OCR.AliYun.AccessKeySecret)
	if _err != nil {
		// 创建失败
		sendToUser(req.AgentID, req.UserID, "图片识别客户端创建失败失败:"+_err.Error(), l.svcCtx.Config)
		return false
	}

	// 进行图片识别
	txt, err := aliocr.OcrImage2Txt(msg, ocrCli)
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
