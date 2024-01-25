package logic

import (
	"chat/common/gemini"
	"context"
	"encoding/json"
	"github.com/pkg/errors"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type KnowledgeSegmentsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKnowledgeSegmentsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KnowledgeSegmentsUpdateLogic {
	return &KnowledgeSegmentsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KnowledgeSegmentsUpdateLogic) KnowledgeSegmentsUpdate(req *types.KnowledgeSegmentsUpdateReq) (resp *types.KnowledgeSegmentsUpdateReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	table := l.svcCtx.Knowledge.KnowledgeUnitSegment
	segment, err := table.WithContext(l.ctx).Where(table.ID.Eq(req.ID)).First()
	if err != nil {
		return nil, err
	}

	knowledgeTable := l.svcCtx.Knowledge.Knowledge
	// FIRST
	knowledgeModel, err := knowledgeTable.WithContext(l.ctx).
		Where(knowledgeTable.ID.Eq(segment.KnowledgeID)).
		Where(knowledgeTable.UserID.Eq(userId)).
		First()
	if err != nil {
		return nil, errors.New("您无权限访问此知识库")
	}
	knowledgeUnitTable := l.svcCtx.Knowledge.KnowledgeUnit
	// SECOND
	_, err = knowledgeUnitTable.WithContext(l.ctx).
		Where(knowledgeUnitTable.ID.Eq(segment.KnowledgeUnitID)).
		Where(knowledgeUnitTable.KnowledgeID.Eq(knowledgeModel.ID)).
		First()
	if err != nil {
		return nil, errors.New("您无权限访问此知识单元")
	}
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
	// 更新
	embed, err := json.Marshal(embeddingResp.Embedding.Values)
	if err != nil {
		return nil, errors.New("gemini embedding json encode 调用失败")
	}
	_, err = table.WithContext(l.ctx).Where(table.ID.Eq(req.ID)).Updates(
		map[string]interface{}{
			"value":     req.Value,
			"embedding": string(embed),
		})
	if err != nil {
		return nil, errors.New("更新失败")
	}
	return &types.KnowledgeSegmentsUpdateReply{}, nil
}
