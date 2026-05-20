package dify

import "fmt"

// APIError 表示 Dify HTTP 调用返回的非 2xx 响应，由 client.sendJSONRequest 与
// api_workflow.go 内的两条流式/非流式工作流路径返回。
//
//   - Status:  HTTP 状态码（401 / 429 / 4xx / 5xx 等）
//   - Code:    Dify 业务码（如 "invalid_api_key" / "conversation_not_found" /
//              "rate_limit_exceeded"），错误体非 JSON 时为空
//   - Message: Dify 返回的人类可读描述；当错误体无法 JSON 解码时退化为原始 body
//
// 适配器层（chat/common/aibackend/dify）用 errors.As(&APIError{}) 来精确分类，
// 不再依赖脆弱的字符串匹配。
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("dify HTTP %d: [%s] %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("dify HTTP %d: %s", e.Status, e.Message)
}
