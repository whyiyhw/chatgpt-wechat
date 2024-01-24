package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeListLogic {
	return &KnowledgeListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeListLogic) KnowledgeList(req *types.KnowledgeListReq) (resp *types.KnowledgeListReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	offset := (req.Page - 1) * req.PageSize
	table := l.svcCtx.Knowledge.Knowledge
	ls, count, findErr := table.WithContext(l.ctx).Where(table.UserID.Eq(userId)).Order(table.ID.Desc()).FindByPage(offset, req.PageSize)
	if findErr != nil {
		return nil, findErr
	}
	res := &types.KnowledgeListReply{
		List:  make([]types.Knowledge, 0),
		Total: int(count),
	}
	for _, v := range ls {
		res.List = append(res.List, types.Knowledge{
			ID:         v.ID,
			Name:       v.Name,
			Avatar:     v.Avatar,
			Desc:       v.Desc,
			CreateTime: v.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdateTime: v.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return res, nil
}
