package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeCreateLogic {
	return &KnowledgeCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeCreateLogic) KnowledgeCreate(req *types.KnowledgeCreateReq) (resp *types.KnowledgeCreateReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	// bot create
	table := l.svcCtx.Knowledge.Knowledge

	err = table.WithContext(l.ctx).Create(&model.Knowledge{
		Name:   req.Name,
		Desc:   req.Desc,
		Avatar: req.Avatar,
		UserID: userId,
	})
	if err != nil {
		return nil, err
	}
	// 拿到存入的ID
	knowledge := new(model.Knowledge)
	knowledge, err = table.WithContext(l.ctx).
		Where(table.UserID.Eq(userId)).
		Where(table.Name.Eq(req.Name)).
		Order(table.ID.Desc()).
		First()
	if err != nil {
		return nil, err
	}

	return &types.KnowledgeCreateReply{
		ID: knowledge.ID,
	}, nil
}
