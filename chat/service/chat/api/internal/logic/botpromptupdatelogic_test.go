package logic

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

func TestBotPromptUpdateLogic_BotPromptUpdate(t *testing.T) {
	conf.MustLoad(*configFile, &c)
	ctx := context.WithValue(context.Background(), "userId", json.Number("1"))
	// 找到一个存在的bot
	testServ := svc.NewServiceContext(c)
	first, err := testServ.BotModel.Bot.WithContext(ctx).First()
	if err != nil {
		t.Errorf("BotUpdate() error = %v", err)
		return
	}
	type fields struct {
		Logger logx.Logger
		ctx    context.Context
		svcCtx *svc.ServiceContext
	}
	type args struct {
		req *types.BotPromptUpdateReq
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp *types.BotPromptUpdateReply
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
				req: &types.BotPromptUpdateReq{
					ID:     first.ID,
					Prompt: "go-zero master" + time.Now().String(),
				},
			},
			wantResp: nil,
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &BotPromptUpdateLogic{
				Logger: tt.fields.Logger,
				ctx:    tt.fields.ctx,
				svcCtx: tt.fields.svcCtx,
			}
			gotResp, err := l.BotPromptUpdate(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BotPromptUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResp, tt.wantResp) {
				t.Errorf("BotPromptUpdate() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}
