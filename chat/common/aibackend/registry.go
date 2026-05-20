package aibackend

import (
	"fmt"
	"sync"
)

// Factory 根据配置创建一个具体后端实例。
//
// cfg 由各实现自定义结构（一般是从 yaml 解码后的 map[string]any 或具体类型），
// 由实现自行做断言/反序列化。
type Factory func(cfg map[string]any) (AIBackend, error)

var (
	factoriesMu sync.RWMutex
	factories   = map[string]Factory{}
)

// Register 注册一个后端类型的工厂函数。
//
// 各实现包应在 init() 中调用 Register("dify", New) 完成自注册。
// 重复注册同名类型会 panic（提前暴露配置/包初始化顺序问题）。
func Register(typeName string, f Factory) {
	if typeName == "" {
		panic("aibackend: Register with empty type name")
	}
	if f == nil {
		panic("aibackend: Register with nil factory")
	}

	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	if _, exists := factories[typeName]; exists {
		panic(fmt.Sprintf("aibackend: Register called twice for type %q", typeName))
	}
	factories[typeName] = f
}

// New 根据类型名创建一个后端实例。
func New(typeName string, cfg map[string]any) (AIBackend, error) {
	factoriesMu.RLock()
	f, ok := factories[typeName]
	factoriesMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("aibackend: unknown backend type %q", typeName)
	}
	return f(cfg)
}

// RegisteredTypes 返回已注册的所有后端类型名（用于诊断/启动日志）。
func RegisteredTypes() []string {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	names := make([]string, 0, len(factories))
	for k := range factories {
		names = append(names, k)
	}
	return names
}
