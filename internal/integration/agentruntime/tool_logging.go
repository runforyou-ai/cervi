package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/domain"
)

type toolCallContextKey struct{}

type toolCallMetadata struct {
	CallID string
}

// toolExecutionMiddleware 记录普通工具与多模态结果工具的过程，并把可继续处理的错误交回模型。
func toolExecutionMiddleware(recorder *processRecorder) compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				var output *compose.ToolOutput
				message, err := recordToolCall(ctx, recorder, input, func(ctx context.Context) (string, error) {
					var err error
					if output, err = next(ctx, input); err != nil {
						return "", err
					}
					return output.Result, nil
				})
				if message != "" {
					return &compose.ToolOutput{Result: message}, nil
				}
				return output, err
			}
		},
		EnhancedInvokable: func(next compose.EnhancedInvokableToolEndpoint) compose.EnhancedInvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.EnhancedInvokableToolOutput, error) {
				var output *compose.EnhancedInvokableToolOutput
				message, err := recordToolCall(ctx, recorder, input, func(ctx context.Context) (string, error) {
					var err error
					if output, err = next(ctx, input); err != nil {
						return "", err
					}
					return toolResultSummary(output.Result), nil
				})
				if message != "" {
					return &compose.EnhancedInvokableToolOutput{Result: &schema.ToolResult{Parts: []schema.ToolOutputPart{{Type: schema.ToolPartTypeText, Text: message}}}}, nil
				}
				return output, err
			}
		},
	}
}

// recordToolCall 执行一次工具调用并记录状态、结果与日志；普通工具错误编码为交回模型的错误消息返回，执行取消和框架中断原样返回错误。
func recordToolCall(ctx context.Context, recorder *processRecorder, input *compose.ToolInput, call func(context.Context) (string, error)) (string, error) {
	startedAt := time.Now()
	if err := recorder.updateTool(input.CallID, func(call *ToolCall) {
		call.Status, call.StartedAt = domain.AgentToolCallRunning, &startedAt
	}); err != nil {
		return "", err
	}
	runID := runIDFromContext(ctx)
	slog.Info("Agent Tool 调用开始",
		"agent_run_id", runID,
		"tool_name", input.Name,
		"tool_call_id", input.CallID,
	)
	toolContext := context.WithValue(ctx, toolCallContextKey{}, toolCallMetadata{CallID: input.CallID})
	result, err := call(toolContext)
	completedAt := time.Now()
	if updateErr := recorder.updateTool(input.CallID, func(call *ToolCall) {
		call.CompletedAt = &completedAt
		if err != nil {
			message := err.Error()
			call.Status, call.Error = domain.AgentToolCallFailed, &message
		} else {
			call.Status, call.Result = domain.AgentToolCallSucceeded, &result
		}
	}); updateErr != nil {
		return "", updateErr
	}
	attributes := []any{
		"agent_run_id", runID,
		"tool_name", input.Name,
		"tool_call_id", input.CallID,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	}
	if err == nil {
		slog.Info("Agent Tool 调用成功", attributes...)
		return "", nil
	}
	attributes = append(attributes, "error", err)
	slog.Warn("Agent Tool 调用失败", attributes...)
	// 执行取消和框架中断继续向上收尾，普通工具错误成为模型输入。
	_, interrupted := compose.ExtractInterruptInfo(err)
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || interrupted {
		return "", err
	}
	encoded, encodeErr := json.Marshal(struct {
		Error string `json:"error"`
	}{Error: err.Error()})
	if encodeErr != nil {
		return "", encodeErr
	}
	return string(encoded), nil
}

// toolResultSummary 把多模态工具结果转成过程记录中的文本，图片等媒体只记录类型。
func toolResultSummary(result *schema.ToolResult) string {
	if result == nil {
		return ""
	}
	parts := make([]string, 0, len(result.Parts))
	for _, part := range result.Parts {
		switch {
		case part.Type == schema.ToolPartTypeText:
			parts = append(parts, part.Text)
		case part.Image != nil:
			parts = append(parts, "[image "+part.Image.MIMEType+"]")
		default:
			parts = append(parts, "["+string(part.Type)+"]")
		}
	}
	return strings.Join(parts, "\n")
}
