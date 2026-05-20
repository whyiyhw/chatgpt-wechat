package aibackend

import (
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/metric"
)

// metric 命名遵循 Prometheus 习惯：{namespace}_{subsystem}_{name}{labels}。
// 这里全部以 chatgpt_wechat_ai_backend_* 为前缀，便于 dashboard 一眼归类。
const (
	metricsNamespace = "chatgpt_wechat"
	metricsSubsystem = "ai_backend"
)

// 三组核心 metric，定义在包级 var 里，进程启动时一次性注册到 prom registry。
//
//   - Prometheus 未配（Host 留空）时 metric.update 直接 return，
//     所有 Inc/Observe 调用都是零成本，可放心散落埋点。
var (
	// ai_backend_request_total{backend, status}
	//   status: success / unavailable / rate_limited / bad_request /
	//           unauthorized / conversation_not_found / not_implemented / error
	requestTotal = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "request_total",
		Help:      "AIBackend 调用总数（按 backend + status 分桶）",
		Labels:    []string{"backend", "status"},
	})

	// ai_backend_duration_seconds{backend, phase}
	//   phase: stream（完整流式调用从发起到 done/error 的端到端时长）
	//   v1.1 仅暴露 stream 一个 phase；后续如需分阶段（decode / call / writeback）再扩。
	durationSeconds = metric.NewHistogramVec(&metric.HistogramVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "duration_seconds",
		Help:      "AIBackend 调用耗时（秒）",
		Labels:    []string{"backend", "phase"},
		Buckets:   []float64{0.1, 0.3, 0.5, 1, 2, 3, 5, 10, 20, 30, 60, 120, 180},
	})

	// ai_backend_first_token_ms{backend}：流式首 chunk 的毫秒数，
	// 用于判断"用户感知响应速度"——比端到端时长更贴近体验。
	firstTokenMS = metric.NewHistogramVec(&metric.HistogramVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "first_token_ms",
		Help:      "ChatStream 收到首个 chunk 的毫秒数",
		Labels:    []string{"backend"},
		Buckets:   []float64{50, 100, 200, 400, 800, 1500, 3000, 6000, 12000, 30000},
	})
)

// MetricObserveRequest 记录一次请求结束。err == nil 视为成功，否则按 ClassifyStatus 分桶。
func MetricObserveRequest(backend string, err error) {
	requestTotal.Inc(backend, ClassifyStatus(err))
}

// MetricObserveDuration 记录一次调用耗时。phase 取值见 durationSeconds 注释。
func MetricObserveDuration(backend, phase string, d time.Duration) {
	durationSeconds.ObserveFloat(d.Seconds(), backend, phase)
}

// MetricObserveFirstToken 记录首 chunk 的毫秒数（ChatStream 用）。
func MetricObserveFirstToken(backend string, ms int64) {
	firstTokenMS.Observe(ms, backend)
}

// ClassifyStatus 把标准错误映射到 prometheus 友好的短字符串。
//
// 不识别的错误归类到 "error"——保留一个兜底，避免 label 值无限膨胀。
func ClassifyStatus(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, ErrBackendUnavailable):
		return "unavailable"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrBadRequest):
		return "bad_request"
	case errors.Is(err, ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, ErrConversationNotFound):
		return "conversation_not_found"
	case errors.Is(err, ErrNotImplemented):
		return "not_implemented"
	default:
		return "error"
	}
}
