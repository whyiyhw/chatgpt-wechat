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

type BotChatLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotChatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotChatLogic {
	return &BotChatLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotChatLogic) BotChat(req *types.BotChatReq) (resp *types.BotChatReply, err error) {
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
	basePrompt := l.svcCtx.Config.Gemini.Prompt
	// 找到 prompt 进行回复
	e := l.svcCtx.ChatModel.BotsPrompt
	prompt, promptErr := e.WithContext(l.ctx).Where(e.BotID.Eq(req.BotID)).First()
	if promptErr == nil {
		basePrompt = prompt.Prompt
	}
	// 生成一个会话的uuid 唯一标识
	uuidObj, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	conversationId := uuidObj.String()

	// 根据 bot 机器人 找到对应的配置进行回复
	if l.svcCtx.Config.ModelProvider.Company == "gemini" {
		c := gemini.NewChatClient(l.svcCtx.Config.Gemini.Key).
			WithTemperature(l.svcCtx.Config.Gemini.Temperature)
		if l.svcCtx.Config.Proxy.Enable {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
				WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
				WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
		}
		// 从上下文中取出用户对话
		collection := gemini.NewUserContext(
			gemini.GetUserUniqueID(strconv.FormatInt(userId, 10), strconv.FormatInt(req.BotID, 10)),
		).WithModel(c.Model).
			WithPrompt(basePrompt).
			WithClient(c).
			Set(gemini.NewChatContent(req.MSG), "", false)

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
						//websocket 数据 发送 错误信息
						http2websocket.Http2websocket(http2websocket.SendData{
							MsgID:       conversationId,
							Msg:         errInfo,
							MsgType:     http2websocket.MsgTypeError,
							MsgToUserID: strconv.FormatInt(userId, 10),
						})
						return
					}
					collection.Set(gemini.NewChatContent(), messageText, true)
					//// 再去插入数据
					//table := l.svcCtx.ChatModel.Chat
					//_ = table.WithContext(context.Background()).Create(&model.Chat{
					//	AgentID:    req.AgentID,
					//	User:       req.UserID,
					//	ReqContent: req.MSG,
					//	ResContent: messageText,
					//})
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

				collection.Set(gemini.NewChatContent(), messageText, true)

				// 再去插入数据
				//table := l.svcCtx.ChatModel.Chat
				//_ = table.WithContext(context.Background()).Create(&model.Chat{
				//	AgentID:    req.AgentID,
				//	User:       req.UserID,
				//	ReqContent: req.MSG,
				//	ResContent: messageText,
				//})
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

		if l.svcCtx.Config.Proxy.Enable {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
				WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
				WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
		}

		// context
		collection := openai.NewUserContext(
			openai.GetUserUniqueID(strconv.FormatInt(userId, 10), strconv.FormatInt(req.BotID, 10)),
		).WithModel(openai.ChatModel4).WithPrompt(basePrompt).WithClient(c).WithTimeOut(l.svcCtx.Config.Session.TimeOut).
			Set(req.MSG, "", false)

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
						//websocket 数据 发送 错误信息
						http2websocket.Http2websocket(http2websocket.SendData{
							MsgID:       conversationId,
							Msg:         errInfo,
							MsgType:     http2websocket.MsgTypeError,
							MsgToUserID: strconv.FormatInt(userId, 10),
						})
						return
					}
					collection.Set("", messageText, true)
					//// 再去插入数据
					//table := l.svcCtx.ChatModel.Chat
					//_ = table.WithContext(context.Background()).Create(&model.Chat{
					//	AgentID:    req.AgentID,
					//	User:       req.UserID,
					//	ReqContent: req.MSG,
					//	ResContent: messageText,
					//})
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

				collection.Set("", messageText, true)

				// 再去插入数据
				//table := l.svcCtx.ChatModel.Chat
				//_ = table.WithContext(context.Background()).Create(&model.Chat{
				//	AgentID:    req.AgentID,
				//	User:       req.UserID,
				//	ReqContent: req.MSG,
				//	ResContent: messageText,
				//})
			}
		}()
	}

	return &types.BotChatReply{
		MessageID: conversationId,
	}, nil
}
