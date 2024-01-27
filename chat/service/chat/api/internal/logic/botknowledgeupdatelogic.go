package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotKnowledgeUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotKnowledgeUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotKnowledgeUpdateLogic {
	return &BotKnowledgeUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotKnowledgeUpdateLogic) BotKnowledgeUpdate(req *types.BotKnowledgeUpdateReq) (resp *types.BotKnowledgeUpdateReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	table := l.svcCtx.ChatModel.Bot
	_, selectErr := table.WithContext(l.ctx).
		Where(table.ID.Eq(req.BotID), table.UserID.Eq(userId)).First()
	if selectErr != nil {
		return nil, selectErr
	}
	botWithKnowledgeTable := l.svcCtx.ChatModel.BotsWithKnowledge
	// 先删除再新增
	_, _ = botWithKnowledgeTable.WithContext(l.ctx).
		Where(botWithKnowledgeTable.BotID.Eq(req.BotID)).
		Delete()

	insertError := botWithKnowledgeTable.WithContext(l.ctx).Create(&model.BotsWithKnowledge{
		BotID:       req.BotID,
		KnowledgeID: req.KnowledgeID,
	})
	if insertError != nil {
		return nil, insertError
	}
	return &types.BotKnowledgeUpdateReply{}, nil
}
