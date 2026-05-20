package aibackend

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, "success"},
		{"unavailable", ErrBackendUnavailable, "unavailable"},
		{"wrapped-unavailable", fmt.Errorf("upstream: %w", ErrBackendUnavailable), "unavailable"},
		{"rate", ErrRateLimited, "rate_limited"},
		{"bad", ErrBadRequest, "bad_request"},
		{"auth", ErrUnauthorized, "unauthorized"},
		{"conv-404", ErrConversationNotFound, "conversation_not_found"},
		{"notimpl", ErrNotImplemented, "not_implemented"},
		{"unknown", errors.New("weird"), "error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyStatus(c.err); got != c.want {
				t.Errorf("ClassifyStatus(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}

// 三组埋点函数本身就是 metric.update 包裹的零开销函数（Prom 未启用时整段 return），
// 这里只验证它们不 panic、能接受预期参数 —— 真正的注册/暴露由 go-zero 自身的
// metric 包测试覆盖。
func TestMetricsSmokeNoPanic(t *testing.T) {
	MetricObserveRequest("dify-test", nil)
	MetricObserveRequest("dify-test", ErrRateLimited)
	MetricObserveDuration("dify-test", "stream", 0)
	MetricObserveFirstToken("dify-test", 0)
}
