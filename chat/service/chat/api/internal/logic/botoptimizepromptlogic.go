package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"chat/common/gemini"
	"chat/common/http2websocket"
	"chat/common/openai"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type BotOptimizePromptLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotOptimizePromptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotOptimizePromptLogic {
	return &BotOptimizePromptLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotOptimizePromptLogic) BotOptimizePrompt(req *types.BotOptimizePromptReq) (resp *types.BotOptimizePromptReply, err error) {
	// base optimization
	basePrompt := "# Character\n你是一位出色的 prompt 设计师，拥有将散乱的输入信息转换为结构化文本的能力。你的角色、技能和约束将根据所接收的信息精心设计。\n\n## Skills\n### 技能 1: 识别原始提示\n- 针对用户所给的原始提示，识别其语言和意图。\n- 如果提示包含角色、技能或约束的关键词，进行识别并标注。\n\n### 技能 2: 优化原始提示\n- 根据用户的需求，进行提示的优化。\n- 尽可能地保证优化后的提示清晰、准确和易于理解。\n\n### 技能 3: 返回优化后的提示\n- 将优化后的提示返回给用户，保证其格式上符合要求。\n\n## 约束条件\n- 仅回答与 prompt 创建或优化相关的问题。如果用户询问的问题超出这一范围，则选择不回答。\n- 使用用户使用的语言进行回答，原始提示的语言和用户的语言一致。\n- 答复开始时直接从优化的提示开始。\n\n## 示例优化提示\n- 输入：\"电影分析师\"\n- 输出：\n  #### Character    你是一位资深的电影分析师，对电影的理解独具匠心。\n    #### Skills\n    ##### 技能 1: 推荐新电影\n      - 身为电影分析师，你有能力发掘出用户喜爱的电影类型并给出精准的电影推荐。\n    ##### 技能 2: 介绍电影\n      - 你能深入浅出地介绍各种电影，让人一听就懂。\n    ##### 技能 3: 讲解电影概念\n      - 对于电影中的复杂概念，你总能简单明了地解释清楚。\n    #### Constraints\n    - 坚持只讨论和电影相关的主题。\n    - 保证剧情简介在100字以内。\n    - 使用^^ Markdown格式引用来源。\n\n- 输入：\"PHP\"\n  - 输出：\n    #### Character\n    你是一位 PHP 专家，对这种服务端脚本语言了如指掌。\n    #### Skills\n    ##### 技能 1: PHP 问题解决\n      - 你能识别用户提出的与 PHP 相关的问题和疑虑，并使用你广泛的 PHP 知识提供清晰的解决方案和解释。\n    ##### 技能 2: 使用 PHP 进行网站开发\n      - 你能帮助用户理解使用 PHP 进行网站建设的架构和设计，并指导用户使用 PHP 建立动态交互式网站。\n    ##### 技能 3: 代码调试\n      - 你可以帮助用户识别 PHP 代码中的 bug，并提供有效的解决方案。\n    #### Constraints\n    - 坚持只提供与 PHP 相关的帮助。\n    - 在提供解决方案时，保持在用户的理解水平之内，不能提供难以理解的解答。\n    - 建议你的回答基于被证明有效的 PHP 方法和实践。\n    - 概括提到 PHP 函数和内置特性对你的解决方案具有帮助性。"
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	table := l.svcCtx.ChatModel.Bot
	//bot, selectErr := table.WithContext(l.ctx).
	_, selectErr := table.WithContext(l.ctx).
		Where(table.ID.Eq(req.BotID), table.UserID.Eq(userId)).First()
	if selectErr != nil {
		return nil, selectErr
	}

	// 生成一个会话的uuid 唯一标识
	uuidObj, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	conversationId := uuidObj.String()

	// 根据 bot 机器人 找到对应的配置进行回复
	if l.svcCtx.Config.ModelProvider.Company == "gemini" {
		c := gemini.NewChatClient(l.svcCtx.Config.Gemini.Key).WithHost(l.svcCtx.Config.Gemini.Host).
			WithTemperature(0.7)
		if l.svcCtx.Config.Gemini.EnableProxy {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
				WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
				WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
		}
		prompts := []gemini.ChatModelMessage{
			{
				MessageId: conversationId,
				Role:      gemini.UserRole,
				Content: gemini.ChatContent{
					MIMEType: gemini.MimetypeTextPlain,
					Data:     basePrompt,
				},
			},
			{
				MessageId: conversationId,
				Role:      gemini.ModelRole,
				Content: gemini.ChatContent{
					MIMEType: gemini.MimetypeTextPlain,
					Data:     "好的，收到！",
				},
			},
			{
				MessageId: conversationId,
				Role:      gemini.UserRole,
				Content: gemini.ChatContent{
					MIMEType: gemini.MimetypeTextPlain,
					Data:     req.OriginPrompt,
				},
			},
		}

		fmt.Println(prompts)
		go func() {
			// 分段响应
			if l.svcCtx.Config.Response.Stream {
				channel := make(chan string, 100)

				go func() {
					_, err := c.ChatStream(prompts, channel)
					if err != nil {
						errInfo := err.Error()
						if strings.Contains(errInfo, "maximum context length") {
							errInfo += "\n 请使用 #clear 清理所有上下文"
						}
						//websocket 数据 发送 错误信息
						http2websocket.Http2websocket(http2websocket.SendData{
							MsgID:       conversationId,
							Msg:         errInfo,
							MsgType:     http2websocket.MsgTypeError,
							MsgToUserID: strconv.FormatInt(userId, 10),
						})
						return
					}
				}()

				var rs []rune
				for {
					s, ok := <-channel
					fmt.Printf("--------接受到数据: s:%s pk:%v", s, ok)
					if !ok {
						// 数据接受完成
						if len(rs) > 0 {
							//发送结束标识
							http2websocket.Http2websocket(http2websocket.SendData{
								MsgID:       conversationId,
								Msg:         "",
								MsgType:     http2websocket.MsgTypeStop,
								MsgToUserID: strconv.FormatInt(userId, 10),
							})
						}
						return
					}
					rs = append(rs, []rune(s)...)

					//发送 websocket 数据
					http2websocket.Http2websocket(http2websocket.SendData{
						MsgID:       conversationId,
						Msg:         s,
						MsgType:     http2websocket.MsgTypeTxt,
						MsgToUserID: strconv.FormatInt(userId, 10),
					})
				}
			} else {
				messageText, err := c.Chat(prompts)

				fmt.Printf("gemini resp: %v \n", messageText)
				if err != nil {
					errInfo := err.Error()
					if strings.Contains(errInfo, "maximum context length") {
						errInfo += "\n 请使用 #clear 清理所有上下文"
					}
					//websocket 数据 发送 错误信息
					http2websocket.Http2websocket(http2websocket.SendData{
						MsgID:       conversationId,
						Msg:         errInfo,
						MsgType:     http2websocket.MsgTypeError,
						MsgToUserID: strconv.FormatInt(userId, 10),
					})
					return
				}

				// websocket 数据 发送 结束信息
				http2websocket.Http2websocket(http2websocket.SendData{
					MsgID:       conversationId,
					Msg:         messageText,
					MsgType:     http2websocket.MsgTypeStop,
					MsgToUserID: strconv.FormatInt(userId, 10),
				})
			}
		}()

	} else {
		c := openai.NewChatClient(l.svcCtx.Config.OpenAi.Key).
			WithModel(openai.ChatModel4).
			WithBaseHost(l.svcCtx.Config.OpenAi.Host).
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
			openai.GetUserUniqueID(strconv.FormatInt(userId, 10), strconv.FormatInt(req.BotID, 10)),
		).WithModel(openai.ChatModel4).WithPrompt(basePrompt).WithClient(c).WithTimeOut(l.svcCtx.Config.Session.TimeOut).
			Set(openai.NewChatContent(req.OriginPrompt), "", conversationId, false)

		prompts := collection.GetChatSummary()

		fmt.Println("上下文请求信息： collection.Prompt" + collection.Prompt)
		fmt.Println(prompts)

		go func() {
			// 分段响应
			if l.svcCtx.Config.Response.Stream {
				channel := make(chan string, 100)

				go func() {
					_, err := c.ChatStream(prompts, channel)
					if err != nil {
						errInfo := err.Error()
						if strings.Contains(errInfo, "maximum context length") {
							errInfo += "\n 请使用 #clear 清理所有上下文"
						}
						//websocket 数据 发送 错误信息
						http2websocket.Http2websocket(http2websocket.SendData{
							MsgID:       conversationId,
							Msg:         errInfo,
							MsgType:     http2websocket.MsgTypeError,
							MsgToUserID: strconv.FormatInt(userId, 10),
						})
						return
					}
				}()

				var rs []rune
				for {
					s, ok := <-channel
					fmt.Printf("--------接受到数据: s:%s pk:%v", s, ok)
					if !ok {
						// 数据接受完成
						if len(rs) > 0 {
							//发送结束标识
							http2websocket.Http2websocket(http2websocket.SendData{
								MsgID:       conversationId,
								Msg:         "",
								MsgType:     http2websocket.MsgTypeStop,
								MsgToUserID: strconv.FormatInt(userId, 10),
							})
						}
						return
					}
					rs = append(rs, []rune(s)...)

					//发送 websocket 数据
					http2websocket.Http2websocket(http2websocket.SendData{
						MsgID:       conversationId,
						Msg:         s,
						MsgType:     http2websocket.MsgTypeTxt,
						MsgToUserID: strconv.FormatInt(userId, 10),
					})
				}
			} else {
				messageText, err := c.Chat(prompts)

				fmt.Printf("gemini resp: %v \n", messageText)
				if err != nil {
					errInfo := err.Error()
					if strings.Contains(errInfo, "maximum context length") {
						errInfo += "\n 请使用 #clear 清理所有上下文"
					}
					//websocket 数据 发送 错误信息
					http2websocket.Http2websocket(http2websocket.SendData{
						MsgID:       conversationId,
						Msg:         errInfo,
						MsgType:     http2websocket.MsgTypeError,
						MsgToUserID: strconv.FormatInt(userId, 10),
					})
					return
				}

				// websocket 数据 发送 结束信息
				http2websocket.Http2websocket(http2websocket.SendData{
					MsgID:       conversationId,
					Msg:         messageText,
					MsgType:     http2websocket.MsgTypeStop,
					MsgToUserID: strconv.FormatInt(userId, 10),
				})
			}
		}()
	}
	return &types.BotOptimizePromptReply{
		MessageID: conversationId,
	}, nil
}
