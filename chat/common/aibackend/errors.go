package aibackend

import "errors"

// 标准错误，便于网关层做统一映射与重试/降级判定。
var (
	// ErrBackendUnavailable 后端不可达（5xx / 超时 / 连接失败），可触发主备切换
	ErrBackendUnavailable = errors.New("aibackend: backend unavailable")

	// ErrBadRequest 请求参数有误（4xx），不应重试
	ErrBadRequest = errors.New("aibackend: bad request")

	// ErrUnauthorized 鉴权失败
	ErrUnauthorized = errors.New("aibackend: unauthorized")

	// ErrRateLimited 触发限流
	ErrRateLimited = errors.New("aibackend: rate limited")

	// ErrConversationNotFound 会话不存在（一般需要新建并重试一次）
	ErrConversationNotFound = errors.New("aibackend: conversation not found")

	// ErrNotImplemented 后端未实现该能力
	ErrNotImplemented = errors.New("aibackend: not implemented")
)

// IsRetryable 判断错误是否可在备用后端上重试。
//
// 仅在网络/服务端类错误（Unavailable / RateLimited）时返回 true；
// 请求参数错误、鉴权错误等不应重试。
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrBackendUnavailable):
		return true
	case errors.Is(err, ErrRateLimited):
		return true
	}
	return false
}
