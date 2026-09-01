//go:build server

package agentruntime

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloudwego/eino/compose"
)

type toolCallContextKey struct{}

type toolCallMetadata struct {
	CallID string
}

// toolLoggingMiddleware 记录非流式 Tool 的实际执行结果，不记录参数或返回内容。
func toolLoggingMiddleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				startedAt := time.Now()
				runID := runIDFromContext(ctx)
				slog.Info("Agent Tool 调用开始",
					"agent_run_id", runID,
					"tool_name", input.Name,
					"tool_call_id", input.CallID,
				)
				toolContext := context.WithValue(ctx, toolCallContextKey{}, toolCallMetadata{CallID: input.CallID})
				output, err := next(toolContext, input)
				attributes := []any{
					"agent_run_id", runID,
					"tool_name", input.Name,
					"tool_call_id", input.CallID,
					"duration_ms", time.Since(startedAt).Milliseconds(),
				}
				if err != nil {
					attributes = append(attributes, "error", err)
					slog.Warn("Agent Tool 调用失败", attributes...)
					return nil, err
				}
				slog.Info("Agent Tool 调用成功", attributes...)
				return output, nil
			}
		},
	}
}

// toolCallMetadataFromContext 返回当前 Tool 实现可用于安全日志的调用标识。
func toolCallMetadataFromContext(ctx context.Context) toolCallMetadata {
	metadata, _ := ctx.Value(toolCallContextKey{}).(toolCallMetadata)
	return metadata
}
