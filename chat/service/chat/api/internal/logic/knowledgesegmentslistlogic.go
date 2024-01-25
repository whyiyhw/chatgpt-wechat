package logic

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeSegmentsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeSegmentsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeSegmentsListLogic {
	return &KnowledgeSegmentsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeSegmentsListLogic) KnowledgeSegmentsList(req *types.KnowledgeSegmentsListReq) (resp *types.KnowledgeSegmentsListReply, err error) {
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

	knowledgeUnitTable := l.svcCtx.Knowledge.KnowledgeUnit
	_, err = knowledgeUnitTable.WithContext(l.ctx).
		Where(knowledgeUnitTable.KnowledgeID.Eq(req.KnowledgeID), knowledgeUnitTable.ID.Eq(req.KnowledgeUnitID)).First()
	if err != nil {
		return nil, errors.New("知识库单元不存在")
	}

	table := l.svcCtx.Knowledge.KnowledgeUnitSegment
	offset := (req.Page - 1) * req.PageSize
	// FIRST
	res, count, err := table.WithContext(l.ctx).Where(table.KnowledgeUnitID.Eq(req.KnowledgeUnitID)).
		Order(table.ID.Desc()).FindByPage(offset, req.PageSize)

	if err != nil {
		return nil, errors.New("知识库单元不存在")
	}
	reply := &types.KnowledgeSegmentsListReply{
		List:  make([]types.KnowledgeSegment, 0),
		Total: int(count),
	}
	for _, v := range res {
		reply.List = append(reply.List, types.KnowledgeSegment{
			ID:              v.ID,
			KnowledgeUnitID: v.KnowledgeUnitID,
			KnowledgeID:     v.KnowledgeID,
			Value:           v.Value,
			CreateTime:      v.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdateTime:      v.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return reply, nil
}
