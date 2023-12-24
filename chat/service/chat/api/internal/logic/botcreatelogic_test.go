package logic

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

func TestBotCreateLogic_BotCreate(t *testing.T) {
	conf.MustLoad(*configFile, &c)

	type fields struct {
		Logger logx.Logger
		ctx    context.Context
		svcCtx *svc.ServiceContext
	}
	type args struct {
		req *types.BotCreateReq
	}
	ctx := context.WithValue(context.Background(), "userId", json.Number("1"))

	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp *types.BotCreateReply
		wantErr  bool
	}{
		// Add test cases.
		{
			name: "success",
			fields: fields{
				Logger: logx.WithContext(ctx),
				ctx:    ctx,
				svcCtx: svc.NewServiceContext(c),
			},
			args: args{
				req: &types.BotCreateReq{
					Name:   "name-test",
					Avatar: "avatar test",
					Desc:   "desc test",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &BotCreateLogic{
				Logger: tt.fields.Logger,
				ctx:    tt.fields.ctx,
				svcCtx: tt.fields.svcCtx,
			}
			gotResp, err := l.BotCreate(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BotCreate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResp, tt.wantResp) {
				t.Errorf("BotCreate() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}
