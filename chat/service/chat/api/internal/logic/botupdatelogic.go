package logic

import (
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"
	"context"
	"encoding/json"

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
	_, selectErr := table.WithContext(l.ctx).
		Where(table.ID.Eq(req.ID), table.UserID.Eq(userId)).First()
	if selectErr != nil {
		return nil, selectErr
	}
	// 更新机器人信息
	_, err = table.WithContext(l.ctx).
		Where(table.ID.Eq(req.ID), table.UserID.Eq(userId)).
		Updates(model.Bot{
			Desc:   req.Desc,
			Name:   req.Name,
			Avatar: req.Avatar,
			UserID: userId,
		})
	if err != nil {
		return nil, err
	}

	return nil, nil
}
