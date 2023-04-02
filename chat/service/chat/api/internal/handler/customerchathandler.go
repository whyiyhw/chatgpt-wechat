package handler

import (
	"fmt"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"chat/common/response"
	"chat/common/validator"
	"chat/service/chat/api/internal/logic"
	"chat/service/chat/api/internal/svc"
	"chat/service/chat/api/internal/types"
)

func CustomerChatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CustomerChatReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(r, w, err)
			return
		}
		// validate check
		if err := validator.Validate.StructCtx(r.Context(), req); err != nil {
			errMap := validator.Translate(err, &req)
			for _, errFormat := range errMap {
				response.ParamError(r, w, fmt.Errorf(errFormat))
				return
			}
			response.ParamError(r, w, err)
			return
		}

		l := logic.NewCustomerChatLogic(r.Context(), svcCtx)
		resp, err := l.CustomerChat(&req)
		response.Response(r, w, resp, err)
	}
}
