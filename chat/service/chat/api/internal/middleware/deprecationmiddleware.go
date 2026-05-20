// Package middleware
//
// DeprecationMiddleware 给 v1.2 计划移除的端点响应里附加 Deprecation / Sunset / Link
// 三个 header，方便前端/接入方提前感知并迁移。详见 doc/PRD-refactor.md §4.1.3。
package middleware

import (
	"fmt"
	"net/http"
	"strings"
)

// deprecatedRoute 一条已废弃路由的匹配规则。
type deprecatedRoute struct {
	Prefix      string // path 前缀匹配
	Replacement string // 推荐迁移到的方案
	Removal     string // 计划移除版本
}

// deprecatedRoutes PRD §4.1.3 列表的可观察化版本，按前缀匹配。
// 改动该列表时需要同步 doc/PRD-refactor.md。
var deprecatedRoutes = []deprecatedRoute{
	{Prefix: "/api/knowledge/", Replacement: "Dify 数据集", Removal: "v1.2"},
	{Prefix: "/api/bot/knowledge/", Replacement: "Dify 数据集", Removal: "v1.2"},
	{Prefix: "/api/bot/prompt/optimize", Replacement: "后端自带 Prompt 优化", Removal: "v1.2"},
	{Prefix: "/api/bot/explore/list", Replacement: "后端应用市场", Removal: "v1.2"},
}

type DeprecationMiddleware struct{}

func NewDeprecationMiddleware() *DeprecationMiddleware {
	return &DeprecationMiddleware{}
}

// Handle 实现 rest.Middleware 签名，可直接挂到 server.Use(...)。
func (m *DeprecationMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if rule := matchDeprecated(r.URL.Path); rule != nil {
			// Deprecation: 表示 API 已弃用（draft-ietf-httpapi-deprecation-header）
			w.Header().Set("Deprecation", "true")
			// Sunset (RFC 8594): 计划下线时间/版本
			w.Header().Set("Sunset", rule.Removal)
			// Link 用 rel="successor-version" 指向迁移指引，alt 写人类可读替代说明
			w.Header().Set(
				"Link",
				fmt.Sprintf(
					`<https://github.com/whyiyhw/chatgpt-wechat/blob/main/doc/PRD-refactor.md>; rel="successor-version"; alt="%s"`,
					rule.Replacement,
				),
			)
		}
		next(w, r)
	}
}

func matchDeprecated(path string) *deprecatedRoute {
	for i := range deprecatedRoutes {
		if strings.HasPrefix(path, deprecatedRoutes[i].Prefix) {
			return &deprecatedRoutes[i]
		}
	}
	return nil
}
