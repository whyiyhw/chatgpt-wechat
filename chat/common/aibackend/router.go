package aibackend

import (
	"encoding/json"
	"fmt"
)

// BackendInstanceConfig 单个 backend 的 yaml 配置；所有 backend 类型的字段并集，
// 由各实现按需取用。yaml 形态见 doc/PRD-refactor.md §6.2。
//
// 字段全部 optional，避免 go-zero conf 在严格模式下因不同 backend 类型缺字段而失败。
type BackendInstanceConfig struct {
	Name          string `json:",optional"`
	Type          string `json:",optional"` // dify | openclaw | openai | http
	BaseURL       string `json:",optional"`
	APIKey        string `json:",optional"`
	Timeout       string `json:",optional"` // duration string, e.g. "60s"
	StreamTimeout string `json:",optional"`
	Model         string `json:",optional"` // 主要给 openai/deepseek 类直连 backend 用
}

// RouteRuleConfig 单条路由规则。
//
// Match 是任意 key:value 集合（bot_id / tenant_id / channel 等），匹配语义由
// Router 在 PR #5/#6 接通调用链时实现，v1.1 RC 阶段只做"配置就位"。
type RouteRuleConfig struct {
	Match    map[string]any `json:",optional"`
	Primary  string         `json:",optional"`
	Fallback []string       `json:",optional"`
}

// RouterConfig AIBackend.* 顶层配置。
type RouterConfig struct {
	Default  string                  `json:",optional"`
	Backends []BackendInstanceConfig `json:",optional"`
	Routes   []RouteRuleConfig       `json:",optional"`
}

// Router 持有所有已实例化的 backend，按名字/路由提供查找。
//
// v1.1 RC 阶段只实现 Default() + Get(name)。Routes 字段已落到结构里但暂不消费，
// 等 PR #5（chatlogic 切接口）/ PR #6（灰度）再接路由匹配逻辑。
type Router struct {
	defaultName string
	backends    map[string]AIBackend
	routes      []RouteRuleConfig
}

// NewRouter 根据配置实例化所有 backend。
//
//   - cfg 全空时返回一个可用但空的 Router；Default() / Get() 都返回 nil。
//   - 任意一个 backend Factory 失败 → 整个 Router 创建失败，启动期就把
//     配置错误暴露出来，避免后续运行时才发现问题。
func NewRouter(cfg RouterConfig) (*Router, error) {
	r := &Router{
		defaultName: cfg.Default,
		backends:    make(map[string]AIBackend, len(cfg.Backends)),
		routes:      cfg.Routes,
	}
	for _, bc := range cfg.Backends {
		if bc.Name == "" {
			return nil, fmt.Errorf("aibackend: Backend entry missing Name (Type=%q)", bc.Type)
		}
		if bc.Type == "" {
			return nil, fmt.Errorf("aibackend: Backend %q missing Type", bc.Name)
		}
		if _, dup := r.backends[bc.Name]; dup {
			return nil, fmt.Errorf("aibackend: duplicate backend name %q", bc.Name)
		}
		m, err := toFactoryMap(bc)
		if err != nil {
			return nil, fmt.Errorf("aibackend: encode config for %q: %w", bc.Name, err)
		}
		b, err := New(bc.Type, m)
		if err != nil {
			return nil, fmt.Errorf("aibackend: instantiate %q (type=%s): %w", bc.Name, bc.Type, err)
		}
		r.backends[bc.Name] = b
	}
	if cfg.Default != "" {
		if _, ok := r.backends[cfg.Default]; !ok {
			return nil, fmt.Errorf("aibackend: default backend %q not present in Backends", cfg.Default)
		}
	}
	return r, nil
}

// Get 按名字取一个 backend；不存在返回 nil。
func (r *Router) Get(name string) AIBackend {
	if r == nil {
		return nil
	}
	return r.backends[name]
}

// Default 返回默认 backend；未配 Default 或 backends 为空时返回 nil。
func (r *Router) Default() AIBackend {
	if r == nil || r.defaultName == "" {
		return nil
	}
	return r.backends[r.defaultName]
}

// Names 列出所有 backend 实例名，用于启动日志 / 诊断。
func (r *Router) Names() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.backends))
	for n := range r.backends {
		names = append(names, n)
	}
	return names
}

// toFactoryMap 把强类型 BackendInstanceConfig 转成 map[string]any，
// 喂给已注册的 Factory（保持 PR #1 既定的 Factory(cfg map[string]any) 接口不变）。
//
// 同时把空字符串字段 drop 掉，让 Factory 走自己的默认值分支而不是"显式空"。
func toFactoryMap(c BackendInstanceConfig) (map[string]any, error) {
	bs, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(bs, &m); err != nil {
		return nil, err
	}
	for k, v := range m {
		if s, ok := v.(string); ok && s == "" {
			delete(m, k)
		}
	}
	return m, nil
}
