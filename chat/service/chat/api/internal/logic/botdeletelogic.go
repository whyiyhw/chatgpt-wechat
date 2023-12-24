package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotDeleteLogic {
	return &BotDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotDeleteLogic) BotDelete(req *types.BotDeleteReq) (resp *types.BotDeleteReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	// delete bot by userid and bot id
	table := l.svcCtx.ChatModel.Bot

	_, selectErr := table.WithContext(l.ctx).
		Where(table.ID.Eq(req.ID), table.UserID.Eq(userId)).First()
	if selectErr != nil {
		return nil, selectErr
	}
	_, err = table.WithContext(l.ctx).
		Where(table.ID.Eq(req.ID), table.UserID.Eq(userId)).Limit(1).Delete()
	if err != nil {
		return nil, err
	}
	return nil, nil
}
