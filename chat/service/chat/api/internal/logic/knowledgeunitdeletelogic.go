package logic

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeUnitDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeUnitDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeUnitDeleteLogic {
	return &KnowledgeUnitDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeUnitDeleteLogic) KnowledgeUnitDelete(req *types.KnowledgeUnitDeleteReq) (resp *types.KnowledgeUnitDeleteReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	table := l.svcCtx.Knowledge.KnowledgeUnit

	// FIRST
	res, err := table.WithContext(l.ctx).Where(table.ID.Eq(req.ID)).First()
	if err != nil {
		return nil, errors.New("该知识单元不存在")
	}
	KnowledgeTable := l.svcCtx.Knowledge.Knowledge
	// SECOND
	_, err = KnowledgeTable.WithContext(l.ctx).
		Where(KnowledgeTable.ID.Eq(res.KnowledgeID)).
		Where(KnowledgeTable.UserID.Eq(userId)).
		First()
	if err != nil {
		return nil, errors.New("您无权限访问此知识单元")
	}
	// THIRD
	_, err = table.WithContext(l.ctx).Where(table.ID.Eq(req.ID)).Delete()
	if err != nil {
		return nil, errors.New("删除失败")
	}

	return &types.KnowledgeUnitDeleteReply{}, nil
}
