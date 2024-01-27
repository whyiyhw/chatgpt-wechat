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
	"chat/common/ps"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm/clause"
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

	// 去找到 bot 机器人对应的model 配置
	botWithModelTable := l.svcCtx.ChatModel.BotsWithModel
	// 找到第一个配置
	firstModel, selectModelErr := botWithModelTable.WithContext(l.ctx).
		Where(botWithModelTable.BotID.Eq(req.BotID)).
		First()
	company := l.svcCtx.Config.ModelProvider.Company
	modelName := ""
	var Temperature float32

	if selectModelErr == nil {
		company = firstModel.ModelType
		modelName = firstModel.ModelName
		Temperature = float32(firstModel.Temperature)
	} else {
		if company == "openai" {
			modelName = openai.ChatModel4
			Temperature = l.svcCtx.Config.OpenAi.Temperature
		} else {
			modelName = l.svcCtx.Config.Gemini.Model
			Temperature = l.svcCtx.Config.Gemini.Temperature
		}
	}
	// 去找到 bot 机器人对应的 knowledge 配置
	botWithKnowledgeTable := l.svcCtx.ChatModel.BotsWithKnowledge
	// 找到第一个配置
	withKnowledge, selectKnowledgeErr := botWithKnowledgeTable.WithContext(l.ctx).
		Where(botWithKnowledgeTable.BotID.Eq(req.BotID)).
		First()
	enableKnowledge := false
	knowledgeModel := &model.Knowledge{}
	if selectKnowledgeErr == nil {
		enableKnowledge = true
		// 去找到 knowledge
		knowledgeTable := l.svcCtx.ChatModel.Knowledge
		knowledgeModel, _ = knowledgeTable.WithContext(l.ctx).
			Where(knowledgeTable.ID.Eq(withKnowledge.KnowledgeID)).
			First()
	}

	// 根据 bot 机器人 找到对应的配置进行回复
	if company == "gemini" {
		c := gemini.NewChatClient(l.svcCtx.Config.Gemini.Key).WithHost(l.svcCtx.Config.Gemini.Host).
			WithTemperature(Temperature)
		if l.svcCtx.Config.Gemini.EnableProxy {
			c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
				WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
				WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
		}
		// 从上下文中取出用户对话
		collection := gemini.NewUserContext(
			gemini.GetUserUniqueID(strconv.FormatInt(userId, 10), strconv.FormatInt(req.BotID, 10)),
		).WithModel(modelName).
			WithPrompt(basePrompt).
			WithClient(c).
			Set(gemini.NewChatContent(req.MSG), "", conversationId, false)

		prompts := collection.GetChatSummary()

		if enableKnowledge && knowledgeModel != nil {
			// 去调用 model 判断是否需要调用 knowledge
			collection.WithPrompt(ps.KnowledgePrompt(knowledgeModel.Name, knowledgeModel.Desc))
			enableKnowledgeMessageText, err := c.Chat(collection.GetChatSummary())
			if err == nil {
				res := ps.KnowledgeResponseParse(enableKnowledgeMessageText)
				if res.IsNeedFindKnowledge {
					// 先去 embedding
					embeddingResp, err := c.CreateEmbedding(req.MSG)
					if err == nil {
						// 再 去 knowledgeUnitSegment 通过 embeddingResp 进行查询
						//	db.Clauses(clause.OrderBy{
						//		Expression: clause.Expr{SQL: "embedding <-> ?", Vars: []interface{}{pgvector.NewVector([]float32{1, 1, 1})}},
						//	}).Limit(5).Find(&items)
						db := l.svcCtx.DbEngin
						var ls []model.KnowledgeUnitSegment
						db.Table(l.svcCtx.Knowledge.KnowledgeUnitSegment.TableName()).Clauses(clause.OrderBy{
							Expression: clause.Expr{SQL: "embedding <-> ?", Vars: []interface{}{pgvector.NewVector(embeddingResp.Embedding.Values)}},
						}).Limit(2).Find(&ls)
						contextString := ""
						for _, segment := range ls {
							fmt.Println("embedding 匹配结果为：", segment.Value)
							contextString = contextString + segment.Value + "\n"
						}
						if contextString != "" {
							// 更改 最后一个 prompts 的值
							prompts[len(prompts)-1].Content.Data = contextString + prompts[len(prompts)-1].Content.Data
						}
					}
				}
			}
			// 还原
			collection.WithPrompt(basePrompt)
		}

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
					collection.Set(gemini.NewChatContent(), messageText, conversationId, true)
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

				collection.Set(gemini.NewChatContent(), messageText, conversationId, true)

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
			WithModel(modelName).
			WithBaseHost(l.svcCtx.Config.OpenAi.Host).
			WithOrigin(l.svcCtx.Config.OpenAi.Origin).
			WithEngine(l.svcCtx.Config.OpenAi.Engine).
			WithMaxToken(l.svcCtx.Config.OpenAi.MaxToken).
			WithTemperature(Temperature).
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
			Set(openai.NewChatContent(req.MSG), "", conversationId, false)

		prompts := collection.GetChatSummary()

		if enableKnowledge && knowledgeModel != nil {
			// 去调用 model 判断是否需要调用 knowledge
			collection.WithPrompt(ps.KnowledgePrompt(knowledgeModel.Name, knowledgeModel.Desc))
			enableKnowledgeMessageText, err := c.Chat(collection.GetChatSummary())
			if err == nil {
				res := ps.KnowledgeResponseParse(enableKnowledgeMessageText)
				if res.IsNeedFindKnowledge {
					// 先去 embedding
					cs := gemini.NewChatClient(l.svcCtx.Config.Gemini.Key).WithHost(l.svcCtx.Config.Gemini.Host).
						WithTemperature(Temperature)
					if l.svcCtx.Config.Gemini.EnableProxy {
						cs = cs.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
							WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
							WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
					}
					embeddingResp, err := cs.CreateEmbedding(req.MSG)
					if err == nil {
						// 再 去 knowledgeUnitSegment 通过 embeddingResp 进行查询
						//	db.Clauses(clause.OrderBy{
						//		Expression: clause.Expr{SQL: "embedding <-> ?", Vars: []interface{}{pgvector.NewVector([]float32{1, 1, 1})}},
						//	}).Limit(5).Find(&items)
						db := l.svcCtx.DbEngin
						var ls []model.KnowledgeUnitSegment
						db.Table(l.svcCtx.Knowledge.KnowledgeUnitSegment.TableName()).Clauses(clause.OrderBy{
							Expression: clause.Expr{SQL: "embedding <-> ?", Vars: []interface{}{pgvector.NewVector(embeddingResp.Embedding.Values)}},
						}).Limit(2).Find(&ls)
						contextString := ""
						for _, segment := range ls {
							fmt.Println("embedding 匹配结果为：", segment.Value)
							contextString = contextString + segment.Value + "\n"
						}
						if contextString != "" {
							// 更改 最后一个 prompts 的值
							prompts[len(prompts)-1].Content.Data = contextString + prompts[len(prompts)-1].Content.Data
						}
					}
				}
			}
			// 还原
			collection.WithPrompt(basePrompt)
		}

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
					collection.Set(openai.NewChatContent(), messageText, conversationId, true)
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

				collection.Set(openai.NewChatContent(), messageText, conversationId, true)

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
