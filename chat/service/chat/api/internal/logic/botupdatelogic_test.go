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

func TestBotUpdateLogic_BotUpdate(t *testing.T) {
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
		req *types.BotUpdateReq
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp *types.BotUpdateReply
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
				req: &types.BotUpdateReq{
					Avatar: "avatar-test",
					Desc:   "desc-update" + time.Now().String(),
					ID:     first.ID,
					Name:   "name-update",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &BotUpdateLogic{
				Logger: tt.fields.Logger,
				ctx:    tt.fields.ctx,
				svcCtx: tt.fields.svcCtx,
			}
			gotResp, err := l.BotUpdate(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BotUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResp, tt.wantResp) {
				t.Errorf("BotUpdate() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}
