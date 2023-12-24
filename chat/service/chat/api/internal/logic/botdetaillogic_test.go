package logic

import (
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"context"
	"encoding/json"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"reflect"
	"testing"
)

func TestBotDetailLogic_BotDetail(t *testing.T) {
	conf.MustLoad(*configFile, &c)
	ctx := context.WithValue(context.Background(), "userId", json.Number("1"))
	// 找到一个存在的bot
	testServ := svc.NewServiceContext(c)
	first, err := testServ.BotModel.Bot.WithContext(ctx).First()
	if err != nil {
		t.Errorf("error = %v", err)
		return
	}
	promptStr := ""
	prompt, err := testServ.BotModel.BotsPrompt.WithContext(ctx).Where(testServ.BotModel.BotsPrompt.BotID.Eq(first.ID)).First()
	if err != nil {
		promptStr = ""
	} else {
		promptStr = prompt.Prompt
	}

	type fields struct {
		Logger logx.Logger
		ctx    context.Context
		svcCtx *svc.ServiceContext
	}
	type args struct {
		req *types.BotDetailReq
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp *types.BotDetailReply
		wantErr  bool
	}{
		{
			name: "test",
			fields: fields{
				Logger: logx.WithContext(ctx),
				ctx:    ctx,
				svcCtx: testServ,
			},
			args: args{
				req: &types.BotDetailReq{
					ID: first.ID,
				},
			},
			wantResp: &types.BotDetailReply{
				ID:     first.ID,
				Name:   first.Name,
				Desc:   first.Desc,
				Avatar: first.Avatar,
				Prompt: promptStr,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &BotDetailLogic{
				Logger: tt.fields.Logger,
				ctx:    tt.fields.ctx,
				svcCtx: tt.fields.svcCtx,
			}
			gotResp, err := l.BotDetail(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BotDetail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResp, tt.wantResp) {
				t.Errorf("BotDetail() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}
