package logic

import (
	"chat/common/wecom"
	"context"
	"encoding/json"

	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CustomerListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerListLogic {
	return &CustomerListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CustomerListLogic) CustomerList(req *types.BotCustomListReq) (resp *types.BotCustomListReply, err error) {
	value := l.ctx.Value("userId")
	if userIdNumber, ok := value.(json.Number); ok {
		_, err = userIdNumber.Int64()
		if err != nil {
			return nil, err
		}
	}
	// 去找 客服用户列表
	customerList, err := wecom.GetCustomerList(req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	var respData types.BotCustomListReply
	for _, customer := range customerList {
		respData.List = append(respData.List, &types.BotCustomListDetail{
			OpenKfid:        customer.OpenKfid,
			Name:            customer.Name,
			Avatar:          customer.Avatar,
			ManagePrivilege: customer.ManagePrivilege,
		})
	}
	return &respData, nil
}
