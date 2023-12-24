package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotUpdateLogic {
	return &BotUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotUpdateLogic) BotUpdate(req *types.BotUpdateReq) (resp *types.BotUpdateReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	table := l.svcCtx.ChatModel.Bot
	first, selectErr := table.WithContext(l.ctx).
		Where(table.ID.Eq(req.ID), table.UserID.Eq(userId)).First()
	if selectErr != nil {
		return nil, selectErr
	}
	first.Desc = req.Desc
	first.Name = req.Name
	first.Avatar = req.Avatar
	err = table.WithContext(l.ctx).
		Where(table.ID.Eq(req.ID), table.UserID.Eq(userId)).
		Save(first)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
