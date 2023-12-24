package logic

import (
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"context"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"testing"
)

func TestUserLoginLogic_UserLogin(t *testing.T) {
	conf.MustLoad(*configFile, &c)

	type fields struct {
		Logger logx.Logger
		ctx    context.Context
		svcCtx *svc.ServiceContext
	}
	type args struct {
		req *types.UserLoginReq
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp *types.UserLoginReply
		wantErr  bool
	}{
		{
			name: "success",
			fields: fields{
				Logger: logx.WithContext(context.Background()),
				ctx:    context.Background(),
				svcCtx: svc.NewServiceContext(c),
			},
			args: args{
				req: &types.UserLoginReq{
					Email:    "demo@163.com",
					Password: "demo@163.com",
				},
			},
			wantResp: &types.UserLoginReply{
				Token: "token",
			},
			wantErr: false,
		},
		{
			name: "fail",
			fields: fields{
				Logger: logx.WithContext(context.Background()),
				ctx:    context.Background(),
				svcCtx: svc.NewServiceContext(c),
			},
			args: args{
				req: &types.UserLoginReq{
					Email:    "demo@163.com",
					Password: "demo@163.comxx",
				},
			},
			wantResp: &types.UserLoginReply{
				Token: "token",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &UserLoginLogic{
				Logger: tt.fields.Logger,
				ctx:    tt.fields.ctx,
				svcCtx: tt.fields.svcCtx,
			}
			_, err := l.UserLogin(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("UserLogin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
