package logic

import (
	"chat/common/xerr"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"
	"context"
	"github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

type UserRegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserRegisterLogic {
	return &UserRegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserRegisterLogic) UserRegister(req *types.UserRegisterReq) (resp *types.UserRegisterReply, err error) {
	// 判断用户是否已经注册
	builder := l.svcCtx.UserModel.RowBuilder().Where(squirrel.Eq{"email": req.Email})

	if exist, err := l.svcCtx.UserModel.FindOneByQuery(l.ctx, builder); err != nil && err != model.ErrNotFound {
		return nil, errors.Wrapf(xerr.NewErrCodeMsg(xerr.DBError, "查询用户失败"), "查询用户失败 %v", err)
	} else {
		if exist != nil {
			return nil, errors.Wrapf(xerr.NewErrMsg("用户已经注册"), "用户已经注册 %d", exist.Id)
		}
	}

	// 加密密码
	password := []byte(req.Password)
	hashedPassword, err2 := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Wrapf(xerr.NewErrMsg("密码加密失败"), "密码加密失败 %v", err2)
	}

	// 未注册的用户进行注册操作
	if _, err := l.svcCtx.UserModel.Insert(l.ctx, &model.User{
		Email:    req.Email,
		Name:     req.Name,
		Password: string(hashedPassword),
	}); err != nil {
		return nil, errors.Wrapf(xerr.NewErrMsg("用户注册失败"), "用户注册失败 %v", err)
	}

	return &types.UserRegisterReply{Message: "注册成功，去登录吧~"}, nil
}
