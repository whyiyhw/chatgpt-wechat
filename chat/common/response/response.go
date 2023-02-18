package response

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"

	"chat/common/xerr"
)

type EmptyJson struct{}

func (e EmptyJson) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}

type Body struct {
	Code uint32      `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func Response(r *http.Request, w http.ResponseWriter, resp interface{}, err error) {

	var body Body

	if err != nil {
		//错误返回
		errCode := xerr.ServerFail
		errMsg := xerr.NewErrCode(xerr.ServerFail).GetErrMsg()

		causeErr := errors.Cause(err)                // err类型
		if e, ok := causeErr.(*xerr.CodeError); ok { // 自定义CodeError
			errCode = e.GetErrCode()
			errMsg = e.GetErrMsg()
		}

		logx.WithContext(r.Context()).Errorf("【API-ERR】 : %+v ", err)

		body.Code = errCode
		body.Msg = errMsg
	} else {
		body.Code = xerr.SUCCESS
		body.Msg = xerr.MapErrMsg(body.Code)
	}

	if resp == nil {
		body.Data = EmptyJson{}
	} else {
		// vi是一个空指针类型的值 那么就转成 json
		vi := reflect.ValueOf(resp)
		if vi.Kind() == reflect.Ptr && vi.IsNil() {
			body.Data = EmptyJson{}
		} else {
			body.Data = resp
		}
	}

	logx.WithContext(r.Context()).WithFields(logx.Field("response", body)).Info()

	httpx.OkJson(w, body)
}

// ParamError 参数错误
func ParamError(r *http.Request, w http.ResponseWriter, err error) {
	errMsg := fmt.Sprintf("%s ,%s", xerr.MapErrMsg(xerr.RequestParamError), err.Error())

	Response(r, w, nil, xerr.NewErrCodeMsg(xerr.RequestParamError, errMsg))
}
