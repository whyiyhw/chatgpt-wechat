package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeUpdateLogic {
	return &KnowledgeUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeUpdateLogic) KnowledgeUpdate(req *types.KnowledgeUpdateReq) (resp *types.KnowledgeUpdateReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	table := l.svcCtx.Knowledge.Knowledge

	// FIRST
	_, err = table.WithContext(l.ctx).Where(table.ID.Eq(req.ID)).
		Where(table.UserID.Eq(userId)).First()
	if err != nil {
		return nil, errors.New("该知识库不存在")
	}

	// SECOND
	_, err = table.WithContext(l.ctx).Where(table.ID.Eq(req.ID)).Updates(
		map[string]interface{}{
			"name":   req.Name,
			"desc":   req.Desc,
			"avatar": req.Avatar,
		})
	if err != nil {
		return nil, errors.New("更新失败")
	}

	return &types.KnowledgeUpdateReply{}, nil
}
