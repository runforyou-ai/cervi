//go:build server

package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/runforyou-ai/cervi/internal/domain"
)

type toolCallContextKey struct{}

type toolCallMetadata struct {
	CallID string
}

// toolExecutionMiddleware 记录工具过程，并把可继续处理的错误交回模型。
func toolExecutionMiddleware(recorder *processRecorder) compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				startedAt := time.Now()
				if err := recorder.updateTool(input.CallID, func(call *ToolCall) {
					call.Status, call.StartedAt = domain.AgentToolCallRunning, &startedAt
				}); err != nil {
					return nil, err
				}
				runID := runIDFromContext(ctx)
				slog.Info("Agent Tool 调用开始",
					"agent_run_id", runID,
					"tool_name", input.Name,
					"tool_call_id", input.CallID,
				)
				toolContext := context.WithValue(ctx, toolCallContextKey{}, toolCallMetadata{CallID: input.CallID})
				output, err := next(toolContext, input)
				completedAt := time.Now()
				if updateErr := recorder.updateTool(input.CallID, func(call *ToolCall) {
					call.CompletedAt = &completedAt
					if err != nil {
						message := err.Error()
						call.Status, call.Error = domain.AgentToolCallFailed, &message
					} else {
						call.Status, call.Result = domain.AgentToolCallSucceeded, &output.Result
					}
				}); updateErr != nil {
					return nil, updateErr
				}
				attributes := []any{
					"agent_run_id", runID,
					"tool_name", input.Name,
					"tool_call_id", input.CallID,
					"duration_ms", time.Since(startedAt).Milliseconds(),
				}
				if err != nil {
					attributes = append(attributes, "error", err)
					slog.Warn("Agent Tool 调用失败", attributes...)
					// 执行取消和框架中断继续向上收尾，普通工具错误成为模型输入。
					_, interrupted := compose.ExtractInterruptInfo(err)
					if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || interrupted {
						return nil, err
					}
					encoded, encodeErr := json.Marshal(struct {
						Error string `json:"error"`
					}{Error: err.Error()})
					if encodeErr != nil {
						return nil, encodeErr
					}
					return &compose.ToolOutput{Result: string(encoded)}, nil
				}
				slog.Info("Agent Tool 调用成功", attributes...)
				return output, nil
			}
		},
	}
}
