package logic

import (
	"context"
	"encoding/json"

	"chat/common/xerr"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

type UserDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserDetailLogic {
	return &UserDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserDetailLogic) UserDetail(req *types.UserDetailReq) (resp *types.UserDetailReply, err error) {
	value := l.ctx.Value("userId")

	if userId, ok := value.(json.Number); ok {
		userIdInt, parseErr := userId.Int64()
		if parseErr != nil {
			return nil, errors.Wrapf(xerr.NewErrMsg("您无权限访问-类型转换失败"), "您无权限访问-类型转换失败")
		}
		// 然后去数据库查询
		table := l.svcCtx.UserModel.User
		first, err := table.WithContext(l.ctx).Where(table.ID.Eq(userIdInt)).First()
		if err != nil {
			return nil, errors.Wrapf(xerr.NewErrMsg("您无权限访问"), "您无权限访问")
		}
		return &types.UserDetailReply{
			ID:      first.ID,
			Name:    first.Name,
			Email:   first.Email,
			IsAdmin: first.IsAdmin,
		}, nil
	}

	return nil, errors.Wrapf(xerr.NewErrMsg("您无权限访问"), "您无权限访问")
}
