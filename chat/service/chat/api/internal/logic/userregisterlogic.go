package logic

import (
	"context"

	"chat/common/xerr"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
	"chat/service/chat/model"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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
	userModel := l.svcCtx.UserModel.User
	exist, selectErr := userModel.WithContext(l.ctx).Where(userModel.Email.Eq(req.Email)).First()
	switch {
	case selectErr == nil:
		return nil, errors.Wrapf(xerr.NewErrMsg("用户已经注册"), "用户已经注册 %d", exist.ID)
	case errors.Is(selectErr, gorm.ErrRecordNotFound):
	default:
		return nil, errors.Wrapf(xerr.NewErrCodeMsg(xerr.DBError, "查询用户失败"), "查询用户失败 %v", selectErr)
	}

	// 加密密码
	password := []byte(req.Password)
	hashedPassword, err2 := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Wrapf(xerr.NewErrMsg("密码加密失败"), "密码加密失败 %v", err2)
	}

	// 未注册的用户进行注册操作
	count, _ := userModel.WithContext(l.ctx).Count()
	isAdmin := false
	if count == 0 {
		isAdmin = true
	}
	if err := userModel.WithContext(l.ctx).Create(&model.User{
		Email:    req.Email,
		Name:     req.Name,
		Password: string(hashedPassword),
		IsAdmin:  isAdmin,
	}); err != nil {
		return nil, errors.Wrapf(xerr.NewErrMsg("用户注册失败"), "用户注册失败 %v", err)
	}

	return &types.UserRegisterReply{Message: "注册成功，去登录吧~"}, nil
}
