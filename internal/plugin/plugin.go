package plugin

// Package plugin：Python 插件 Hook（fail-open）。Phase 1 仅预留配置与空实现。
type Client interface {
	Enabled() bool
	// PickModel 可选：返回推荐 model；失败时调用方走默认路由。
	PickModel(payload map[string]any) (map[string]any, error)
}

type Noop struct{}

func (Noop) Enabled() bool { return false }

func (Noop) PickModel(map[string]any) (map[string]any, error) {
	return nil, nil
}
