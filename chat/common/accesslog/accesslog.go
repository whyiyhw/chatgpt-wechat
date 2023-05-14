package accesslog

import (
	"net/http"

	"github.com/zeromicro/go-zero/core/logx"
)

type AccessLog struct {
	Url    string      `json:"url"`
	Method string      `json:"method"`
	Query  string      `json:"query"`
	Body   string      `json:"body"`
	Header http.Header `json:"header"`
	Const  int         `json:"const"`
}

func ToLog(r *http.Request, bodyByte []byte, ms int) {

	l := AccessLog{
		Url:    r.URL.Path,
		Method: r.Method,
		Query:  r.URL.Query().Encode(),
		Body:   string(bodyByte),
		Header: r.Header,
		Const:  ms,
	}

	logx.WithContext(r.Context()).WithFields(logx.Field("request", l)).Info()
}
