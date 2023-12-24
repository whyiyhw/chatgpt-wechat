package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotDetailLogic {
	return &BotDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotDetailLogic) BotDetail(req *types.BotDetailReq) (resp *types.BotDetailReply, err error) {
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
	// 去找 prompt
	e := l.svcCtx.ChatModel.BotsPrompt
	showPrompt := ""
	if prompt, promptErr := e.WithContext(l.ctx).Where(e.BotID.Eq(req.ID)).First(); promptErr == nil {
		showPrompt = prompt.Prompt
	}

	resp = &types.BotDetailReply{
		ID:     first.ID,
		Name:   first.Name,
		Avatar: first.Avatar,
		Desc:   first.Desc,
		Prompt: showPrompt,
	}

	return
}
