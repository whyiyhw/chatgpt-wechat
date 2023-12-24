package logic

import (
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/zeromicro/go-zero/core/logx"
)

type BotPromptUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBotPromptUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BotPromptUpdateLogic {
	return &BotPromptUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BotPromptUpdateLogic) BotPromptUpdate(req *types.BotPromptUpdateReq) (resp *types.BotPromptUpdateReply, err error) {
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
		Where(table.ID.Eq(req.ID), table.UserID.Eq(userId)).First()
	if selectErr != nil {
		return nil, selectErr
	}
	// 去找 prompt
	e := l.svcCtx.ChatModel.BotsPrompt
	prompt, promptErr := e.WithContext(l.ctx).Where(e.BotID.Eq(req.ID)).First()
	if promptErr == nil {
		if prompt.Prompt != req.Prompt {
			_, updateErr := e.WithContext(l.ctx).Where(e.BotID.Eq(req.ID)).Update(e.Prompt, req.Prompt)
			if updateErr != nil {
				return nil, updateErr
			}
		}
	} else if errors.Is(promptErr, gorm.ErrRecordNotFound) {
		// create
		if req.Prompt != "" {
			createErr := e.WithContext(l.ctx).Create(&model.BotsPrompt{
				Prompt: req.Prompt,
				BotID:  req.ID,
			})
			if createErr != nil {
				return nil, createErr
			}
		}
	}
	return nil, nil
}
