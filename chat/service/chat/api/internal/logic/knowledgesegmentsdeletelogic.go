package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeSegmentsDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeSegmentsDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeSegmentsDeleteLogic {
	return &KnowledgeSegmentsDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeSegmentsDeleteLogic) KnowledgeSegmentsDelete(req *types.KnowledgeSegmentsDeleteReq) (resp *types.KnowledgeSegmentsDeleteReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	table := l.svcCtx.Knowledge.KnowledgeUnitSegment

	// FIRST
	res, err := table.WithContext(l.ctx).Where(table.ID.Eq(req.ID)).First()
	if err != nil {
		return nil, errors.New("该知识点不存在")
	}
	KnowledgeTable := l.svcCtx.Knowledge.Knowledge
	// SECOND
	knowledgeModel, err := KnowledgeTable.WithContext(l.ctx).
		Where(KnowledgeTable.ID.Eq(res.KnowledgeID)).
		Where(KnowledgeTable.UserID.Eq(userId)).
		First()
	if err != nil {
		return nil, errors.New("您无权限访问此知识点")
	}
	if knowledgeModel.UserID != userId {
		return nil, errors.New("您无权限访问此知识点")
	}
	// THIRD
	_, err = table.WithContext(l.ctx).Where(table.ID.Eq(req.ID)).Delete()
	if err != nil {
		return nil, errors.New("删除失败")
	}
	return &types.KnowledgeSegmentsDeleteReply{}, nil
}
