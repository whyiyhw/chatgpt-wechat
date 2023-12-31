package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotListLogic {
	return &BotListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotListLogic) BotList(req *types.BotListReq) (resp *types.BotListReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	offset := (req.Page - 1) * req.PageSize
	table := l.svcCtx.ChatModel.Bot
	ls, count, findErr := table.WithContext(l.ctx).Where(table.UserID.Eq(userId)).Order(table.ID.Desc()).FindByPage(offset, req.PageSize)
	if findErr != nil {
		return nil, findErr
	}
	res := &types.BotListReply{
		Page:     req.Page,
		PageSize: req.PageSize,
		List:     make([]*types.BotListDetail, 0),
		Total:    int(count),
	}
	for _, v := range ls {
		res.List = append(res.List, &types.BotListDetail{
			ID:     v.ID,
			Name:   v.Name,
			Avatar: v.Avatar,
			Desc:   v.Desc,
		})
	}
	return res, nil
}
