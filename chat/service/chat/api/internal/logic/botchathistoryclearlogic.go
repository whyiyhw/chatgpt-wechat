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

type BotChatHistoryClearLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotChatHistoryClearLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotChatHistoryClearLogic {
	return &BotChatHistoryClearLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotChatHistoryClearLogic) BotChatHistoryClear(req *types.BotChatHistoryClearReq) (resp *types.BotChatHistoryClearReply, err error) {
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
		// 从上下文中清理用户对话
		uniqueKey := gemini.GetUserUniqueID(strconv.FormatInt(userId, 10), strconv.FormatInt(req.BotID, 10))
		gemini.NewUserContext(uniqueKey).ChatClear(uniqueKey)
	} else {
		// 从上下文中清理用户对话
		openai.NewUserContext(
			openai.GetUserUniqueID(strconv.FormatInt(userId, 10), strconv.FormatInt(req.BotID, 10)),
		).Clear()
	}

	return
}
