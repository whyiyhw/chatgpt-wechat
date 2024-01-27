package logic

import (
	"context"
	"encoding/json"
	"strconv"

	"chat/common/gemini"
	"chat/common/openai"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotChatHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotChatHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotChatHistoryLogic {
	return &BotChatHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotChatHistoryLogic) BotChatHistory(req *types.BotChatHistoryReq) (resp *types.BotChatHistoryReply, err error) {
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

	// 根据 bot 机器人 找到对应的配置进行回复
	if l.svcCtx.Config.ModelProvider.Company == "gemini" {
		c := gemini.NewChatClient(l.svcCtx.Config.Gemini.Key).WithHost(l.svcCtx.Config.Gemini.Host).
			WithTemperature(l.svcCtx.Config.Gemini.Temperature)
		// 从上下文中取出用户对话
		collection := gemini.NewUserContext(
			gemini.GetUserUniqueID(strconv.FormatInt(userId, 10), strconv.FormatInt(req.BotID, 10)),
		).WithModel(c.Model).GetChatHistory()

		response := &types.BotChatHistoryReply{
			List: make([]*types.BotChatWholeReply, 0),
		}
		for _, message := range collection {
			response.List = append(response.List, &types.BotChatWholeReply{
				Role: message.Role,
				Content: types.BotChatContent{
					MimeType: message.Content.MIMEType,
					Data:     message.Content.Data,
				},
			})
		}
		return response, nil
	}
	// openai
	collection := openai.NewUserContext(
		openai.GetUserUniqueID(strconv.FormatInt(userId, 10), strconv.FormatInt(req.BotID, 10)),
	).GetChatSummary()
	response := &types.BotChatHistoryReply{
		List: make([]*types.BotChatWholeReply, 0),
	}
	for _, message := range collection {
		if message.Role == openai.SystemRole {
			continue
		}
		response.List = append(response.List, &types.BotChatWholeReply{
			Role: message.Role,
			Content: types.BotChatContent{
				MimeType: message.Content.MIMEType,
				Data:     message.Content.Data,
			},
		})
	}
	return response, nil
}
