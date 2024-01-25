package logic

import (
	"context"
	"encoding/json"

	"chat/common/gemini"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeSegmentsCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeSegmentsCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeSegmentsCreateLogic {
	return &KnowledgeSegmentsCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeSegmentsCreateLogic) KnowledgeSegmentsCreate(req *types.KnowledgeSegmentsCreateReq) (resp *types.KnowledgeSegmentsCreateReply, err error) {
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
	knowledgeUnitTable := l.svcCtx.Knowledge.KnowledgeUnit
	// SECOND
	knowledgeUnitModel, err := knowledgeUnitTable.WithContext(l.ctx).
		Where(knowledgeUnitTable.ID.Eq(req.KnowledgeUnitID)).
		Where(knowledgeUnitTable.KnowledgeID.Eq(knowledgeModel.ID)).
		First()
	if err != nil {
		return nil, errors.New("您无权限访问此知识单元")
	}
	// THIRD
	// 调用 embedding 的接口
	c := gemini.NewChatClient(l.svcCtx.Config.Gemini.Key).WithHost(l.svcCtx.Config.Gemini.Host)
	if l.svcCtx.Config.Gemini.EnableProxy {
		c = c.WithHttpProxy(l.svcCtx.Config.Proxy.Http).WithSocks5Proxy(l.svcCtx.Config.Proxy.Socket5).
			WithProxyUserName(l.svcCtx.Config.Proxy.Auth.Username).
			WithProxyPassword(l.svcCtx.Config.Proxy.Auth.Password)
	}
	embeddingResp, err := c.CreateEmbedding(req.Value)
	if err != nil {
		return nil, errors.New("gemini embedding 接口调用失败")
	}
	// 四
	embed, err := json.Marshal(embeddingResp.Embedding.Values)
	if err != nil {
		return nil, err
	}
	table := l.svcCtx.Knowledge.KnowledgeUnitSegment
	err = table.WithContext(l.ctx).Create(&model.KnowledgeUnitSegment{
		KnowledgeID:     knowledgeModel.ID,
		KnowledgeUnitID: knowledgeUnitModel.ID,
		Value:           req.Value,
		Embedding:       string(embed),
	})
	if err != nil {
		return nil, errors.New("创建失败")
	}
	return &types.KnowledgeSegmentsCreateReply{}, nil
}
