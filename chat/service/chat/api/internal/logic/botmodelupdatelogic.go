package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotModelUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotModelUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotModelUpdateLogic {
	return &BotModelUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotModelUpdateLogic) BotModelUpdate(req *types.BotModelUpdateReq) (resp *types.BotModelUpdateReply, err error) {
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
	botWithModelTable := l.svcCtx.ChatModel.BotsWithModel
	// 先删除再新增
	_, _ = botWithModelTable.WithContext(l.ctx).
		Where(botWithModelTable.BotID.Eq(req.BotID)).
		Delete()

	insertError := botWithModelTable.WithContext(l.ctx).Create(&model.BotsWithModel{
		BotID:       req.BotID,
		ModelType:   req.ModelType,
		ModelName:   req.ModelName,
		Temperature: req.Temperature,
	})
	if insertError != nil {
		return nil, insertError
	}

	return &types.BotModelUpdateReply{}, nil
}
