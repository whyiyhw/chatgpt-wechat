package logic

import (
	"context"
	"gorm.io/gorm"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"

	"chat/common/xerr"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
)

type UserLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserLoginLogic {
	return &UserLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserLoginLogic) UserLogin(req *types.UserLoginReq) (resp *types.UserLoginReply, err error) {
	// 查询 用户是否存在
	userModel := l.svcCtx.UserModel.User
	res, selectErr := userModel.WithContext(l.ctx).Where(userModel.Email.Eq(req.Email)).First()
	if !errors.Is(selectErr, gorm.ErrRecordNotFound) && selectErr != nil {
		err = errors.Wrapf(xerr.NewErrCodeMsg(xerr.DBError, "查询用户失败"), "查询用户失败 %v", err)
		return
	}
	if errors.Is(selectErr, gorm.ErrRecordNotFound) {
		return nil, errors.Wrapf(xerr.NewErrMsg("账号或密码错误"), "账号或密码错误 %s", req.Email)
	}

	// 验证密码是否正确
	err = bcrypt.CompareHashAndPassword([]byte(res.Password), []byte(req.Password))
	if err != nil {
		return nil, errors.Wrapf(xerr.NewErrMsg("账号或密码错误"), "账号或密码错误 %v", err)
	}

	// 生成 token 并进行响应
	token, tokenErr := l.getJwtToken(
		l.svcCtx.Config.Auth.AccessSecret,
		time.Now().Unix(),
		l.svcCtx.Config.Auth.AccessExpire,
		res.ID,
	)

	if tokenErr != nil {
		return nil, errors.Wrapf(xerr.NewErrMsg("生成 token 失败"), "生成 token 失败 %v", tokenErr)
	}

	return &types.UserLoginReply{Token: token}, nil
}

func (l *UserLoginLogic) getJwtToken(secretKey string, iat, seconds, userId int64) (string, error) {
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + seconds
	claims["iat"] = iat
	claims["userId"] = userId
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims
	return token.SignedString([]byte(secretKey))
}
