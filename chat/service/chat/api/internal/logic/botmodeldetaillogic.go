package logic

import (
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotModelDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotModelDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotModelDetailLogic {
	return &BotModelDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotModelDetailLogic) BotModelDetail(req *types.BotModelDetailReq) (resp *types.BotModelDetailReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	table := l.svcCtx.ChatModel.Bot
	_, selectErr := table.WithContext(l.ctx).
		Where(table.ID.Eq(req.BotID), table.UserID.Eq(userId)).First()
	if selectErr != nil {
		return nil, selectErr
	}
	botWithModelTable := l.svcCtx.ChatModel.BotsWithModel

	// 找到第一个配置
	first, selectErr := botWithModelTable.WithContext(l.ctx).
		Where(botWithModelTable.BotID.Eq(req.BotID)).
		First()
	if selectErr != nil {
		// 去配置中找到默认配置响应
		if l.svcCtx.Config.ModelProvider.Company == "openai" {
			resp = &types.BotModelDetailReply{
				ModelType:   "openai",
				ModelName:   "gpt-4",
				Temperature: float64(l.svcCtx.Config.OpenAi.Temperature),
			}
		} else {
			resp = &types.BotModelDetailReply{
				ModelType:   "gemini",
				ModelName:   l.svcCtx.Config.Gemini.Model,
				Temperature: float64(l.svcCtx.Config.Gemini.Temperature),
			}
		}
		return
	}

	resp = &types.BotModelDetailReply{
		ModelType:   first.ModelType,
		ModelName:   first.ModelName,
		Temperature: first.Temperature,
	}
	return
}
