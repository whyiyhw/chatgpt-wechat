package logic

import (
	"chat/service/chat/model"
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotCreateLogic {
	return &BotCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotCreateLogic) BotCreate(req *types.BotCreateReq) (resp *types.BotCreateReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	// bot create
	table := l.svcCtx.BotModel.Bot

	err = table.WithContext(l.ctx).Create(&model.Bot{
		Name:   req.Name,
		Desc:   req.Desc,
		Avatar: req.Avatar,
		UserID: userId,
	})
	if err != nil {
		return nil, err
	}

	return
}
