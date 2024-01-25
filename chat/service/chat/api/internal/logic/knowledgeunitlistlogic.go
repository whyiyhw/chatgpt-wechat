package logic

import (
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"context"
	"encoding/json"
	"github.com/pkg/errors"

	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeUnitListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeUnitListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeUnitListLogic {
	return &KnowledgeUnitListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeUnitListLogic) KnowledgeUnitList(req *types.KnowledgeUnitListReq) (resp *types.KnowledgeUnitListReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	knowledgeTable := l.svcCtx.Knowledge.Knowledge
	_, err = knowledgeTable.
		WithContext(l.ctx).
		Where(knowledgeTable.UserID.Eq(userId), knowledgeTable.ID.Eq(req.KnowledgeID)).First()
	if err != nil {
		return nil, errors.New("知识库不存在")
	}

	table := l.svcCtx.Knowledge.KnowledgeUnit
	offset := (req.Page - 1) * req.PageSize
	// FIRST
	res, count, err := table.WithContext(l.ctx).Where(table.KnowledgeID.Eq(req.KnowledgeID)).
		Order(table.ID.Desc()).FindByPage(offset, req.PageSize)

	if err != nil {
		return nil, errors.New("知识库单元不存在")
	}
	reply := &types.KnowledgeUnitListReply{
		List:  make([]types.KnowledgeUnit, 0),
		Total: int(count),
	}
	for _, v := range res {
		reply.List = append(reply.List, types.KnowledgeUnit{
			ID:          v.ID,
			KnowledgeID: v.KnowledgeID,
			Name:        v.Name,
			Type:        v.Type,
			Source:      v.Source,
			Enable:      v.Enable,
			CreateTime:  v.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdateTime:  v.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return reply, nil
}
