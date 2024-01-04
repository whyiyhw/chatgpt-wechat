package logic

import (
	"chat/service/chat/model"
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotCustomerUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotCustomerUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotCustomerUpdateLogic {
	return &BotCustomerUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotCustomerUpdateLogic) BotCustomerUpdate(req *types.BotCustomUpdateReq) (resp *types.BotCustomUpdateReply, err error) {
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
	botWithCustomTable := l.svcCtx.ChatModel.BotsWithCustom
	// 先删除再新增
	_, _ = botWithCustomTable.WithContext(l.ctx).
		Where(botWithCustomTable.OpenKfID.Eq(req.OpenKfid)).
		Delete()

	insertError := botWithCustomTable.WithContext(l.ctx).Create(&model.BotsWithCustom{
		BotID:    req.BotID,
		OpenKfID: req.OpenKfid,
	})
	if insertError != nil {
		return nil, insertError
	}

	return &types.BotCustomUpdateReply{}, nil
}
