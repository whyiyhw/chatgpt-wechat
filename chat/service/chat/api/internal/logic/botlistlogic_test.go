package logic

import (
	"context"
	"encoding/json"
	"testing"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

func TestBotListLogic_BotList(t *testing.T) {
	conf.MustLoad(*configFile, &c)
	ctx := context.WithValue(context.Background(), "userId", json.Number("1"))
	testServ := svc.NewServiceContext(c)

	type fields struct {
		Logger logx.Logger
		ctx    context.Context
		svcCtx *svc.ServiceContext
	}
	type args struct {
		req *types.BotListReq
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp *types.BotListReply
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
				req: &types.BotListReq{
					Page:     1,
					PageSize: 10,
				},
			},
			wantResp: nil,
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &BotListLogic{
				Logger: tt.fields.Logger,
				ctx:    tt.fields.ctx,
				svcCtx: tt.fields.svcCtx,
			}
			_, err := l.BotList(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BotList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
