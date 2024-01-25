package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeUnitCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeUnitCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeUnitCreateLogic {
	return &KnowledgeUnitCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeUnitCreateLogic) KnowledgeUnitCreate(req *types.KnowledgeUnitCreateReq) (resp *types.KnowledgeUnitCreateReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	knowledgeTable := l.svcCtx.Knowledge.Knowledge
	// FIRST
	knowledgeModel, err := knowledgeTable.WithContext(l.ctx).
		Where(knowledgeTable.ID.Eq(req.KnowledgeID)).
		Where(knowledgeTable.UserID.Eq(userId)).
		First()
	if err != nil {
		return nil, errors.New("您无权限访问此知识库")
	}
	// SECOND
	table := l.svcCtx.Knowledge.KnowledgeUnit
	err = table.WithContext(l.ctx).Create(&model.KnowledgeUnit{
		KnowledgeID: knowledgeModel.ID,
		Name:        req.Name,
		Type:        "文本",
		Source:      "手动",
		Enable:      true,
	})
	if err != nil {
		return nil, errors.New("创建失败")
	}
	// 拿到存入的ID
	res, err := table.WithContext(l.ctx).
		Where(table.KnowledgeID.Eq(knowledgeModel.ID)).
		Where(table.Name.Eq(req.Name)).
		Order(table.ID.Desc()).First()
	if err != nil {
		return nil, errors.New("创建失败")
	}

	return &types.KnowledgeUnitCreateReply{
		ID: res.ID,
	}, nil
}
