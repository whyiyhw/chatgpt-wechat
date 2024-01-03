package logic

import (
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"
	"context"
	"encoding/json"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotReplicateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotReplicateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotReplicateLogic {
	return &BotReplicateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotReplicateLogic) BotReplicate(req *types.BotReplicateReq) (resp *types.BotReplicateReply, err error) {
	value := l.ctx.Value("userId")
	var userId int64
	if userIdNumber, ok := value.(json.Number); ok {
		userId, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	var newBot *model.Bot
	if req.OriginType == 1 {
		table := l.svcCtx.ChatModel.Bot
		first, selectErr := table.WithContext(l.ctx).
			Where(table.ID.Eq(req.ID), table.UserID.Eq(userId)).First()
		if selectErr != nil {
			return nil, selectErr
		}
		// 去生成新的bot
		newBot = &model.Bot{
			UserID: userId,
			Name:   first.Name,
			Avatar: first.Avatar,
			Desc:   first.Desc,
		}
		insertErr := table.WithContext(l.ctx).Create(newBot)
		if insertErr != nil {
			return nil, insertErr
		}
		// 去找 prompt
		e := l.svcCtx.ChatModel.BotsPrompt
		prompt, promptErr := e.WithContext(l.ctx).Where(e.BotID.Eq(req.ID)).First()
		if promptErr == nil {
			updateErr := e.WithContext(l.ctx).Create(&model.BotsPrompt{
				BotID:  newBot.ID,
				Prompt: prompt.Prompt,
			})
			if updateErr != nil {
				return nil, updateErr
			}
		}
	} else {
		table := l.svcCtx.ChatModel.PromptConfig
		first, selectErr := table.WithContext(l.ctx).
			Where(table.ID.Eq(req.ID)).First()
		if selectErr != nil {
			return nil, selectErr
		}
		// 去生成新的bot
		newBot = &model.Bot{
			UserID: userId,
			Name:   first.Key,
			Avatar: "",
			Desc:   "",
		}
		botTable := l.svcCtx.ChatModel.Bot
		insertErr := botTable.WithContext(l.ctx).Create(newBot)
		if insertErr != nil {
			return nil, insertErr
		}
		// 去找 prompt
		e := l.svcCtx.ChatModel.BotsPrompt
		updateErr := e.WithContext(l.ctx).Create(&model.BotsPrompt{
			BotID:  newBot.ID,
			Prompt: first.Value,
		})
		if updateErr != nil {
			return nil, updateErr
		}
	}

	return &types.BotReplicateReply{
		ID: newBot.ID,
	}, nil
}
