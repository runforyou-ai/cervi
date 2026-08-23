//go:build server

package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/task"
)

// Registry 保存服务端可异步执行的 Action。
type Registry struct {
	handlers map[string]task.Handler
}

// NewRegistry 创建 Action 注册表。
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]task.Handler)}
}

// Register 注册一个原始 JSON Action 处理器。
func (r *Registry) Register(name string, handler task.Handler) error {
	if !actionNamePattern.MatchString(name) {
		return fmt.Errorf("invalid task action name %q", name)
	}
	if handler == nil {
		return fmt.Errorf("task action %q has nil handler", name)
	}
	if _, exists := r.handlers[name]; exists {
		return fmt.Errorf("task action %q already registered", name)
	}
	r.handlers[name] = handler
	return nil
}

// lookup 查找已注册的 Action 处理器。
func (r *Registry) lookup(name string) (task.Handler, bool) {
	handler, exists := r.handlers[name]
	return handler, exists
}

// RegisterJSON 注册使用强类型 JSON 输入的 Action。
func RegisterJSON[T any](registry *Registry, name string, execute func(context.Context, T) error) error {
	return registry.Register(name, func(ctx context.Context, payload json.RawMessage) error {
		var input T
		if err := json.Unmarshal(payload, &input); err != nil {
			return task.Permanent(fmt.Errorf("decode %s payload: %w", name, err))
		}
		return execute(ctx, input)
	})
}
