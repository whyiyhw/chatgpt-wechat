package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotExploreListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotExploreListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotExploreListLogic {
	return &BotExploreListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotExploreListLogic) BotExploreList(req *types.BotExploreListReq) (resp *types.BotExploreListReply, err error) {
	value := l.ctx.Value("userId")
	if userIdNumber, ok := value.(json.Number); ok {
		_, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	offset := (req.Page - 1) * req.PageSize
	table := l.svcCtx.ChatModel.PromptConfig
	ls, count, findErr := table.WithContext(l.ctx).Order(table.ID.Desc()).FindByPage(offset, req.PageSize)
	if findErr != nil {
		return nil, findErr
	}
	res := &types.BotExploreListReply{
		Page:     req.Page,
		PageSize: req.PageSize,
		List:     make([]*types.ExploreListBot, 0),
		Total:    int(count),
	}
	for _, v := range ls {
		res.List = append(res.List, &types.ExploreListBot{
			ID:  v.ID,
			Key: v.Key,
		})
	}
	return res, nil
}
