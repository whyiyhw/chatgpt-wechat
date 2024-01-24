package logic

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeDeleteLogic {
	return &KnowledgeDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeDeleteLogic) KnowledgeDelete(req *types.KnowledgeDeleteReq) (resp *types.KnowledgeDeleteReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	table := l.svcCtx.Knowledge.Knowledge

	// 拿到存入的ID
	_, err = table.WithContext(l.ctx).
		Where(table.UserID.Eq(userId)).
		Where(table.ID.Eq(req.ID)).
		Order(table.ID.Desc()).
		First()
	if err != nil {
		return nil, errors.New("知识库不存在")
	}
	// 删除
	res, err := table.WithContext(l.ctx).
		Where(table.UserID.Eq(userId)).
		Where(table.ID.Eq(req.ID)).
		Delete()
	if err != nil {
		return nil, errors.New("删除失败")
	}
	if res.RowsAffected == 0 {
		return nil, errors.New("删除失败或已被删除")
	}

	return &types.KnowledgeDeleteReply{}, nil
}
