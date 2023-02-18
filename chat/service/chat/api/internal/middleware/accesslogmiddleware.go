package middleware

import (
	"bytes"
	"chat/common/accesslog"
	"io"
	"net/http"
	"time"
)

type AccessLogMiddleware struct {
}

func NewAccessLogMiddleware() *AccessLogMiddleware {
	return &AccessLogMiddleware{}
}

func (m *AccessLogMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Passthrough to next handler if need
		startTime := time.Now().Local()

		// 从 body 中取出数据
		bodyByte, _ := io.ReadAll(r.Body)

		// copy后 还给业务使用
		r.Body = io.NopCloser(bytes.NewBuffer(bodyByte))

		next(w, r)

		endTime := time.Now().Local()
		ms := (endTime.Nanosecond() - startTime.Nanosecond()) / 1000000

		accesslog.ToLog(r, bodyByte, ms)
	}
}
