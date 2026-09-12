//go:build server

package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"unicode"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	fsmiddleware "github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/reduction"
	"github.com/cloudwego/eino/schema"
)

// offloadedResultToolName 是读回转存工具结果的工具名，与本地文件系统工具区分。
const offloadedResultToolName = "read_offloaded_tool_result"

const (
	// toolResultOffloadBytes 单次工具结果超过该字节数时转存，上下文只保留首尾预览，约合一万汉字。
	toolResultOffloadBytes = 30000
	// contextClearWindowPercent 上下文 Token 占模型窗口达到该百分比时清理历史工具调用。
	contextClearWindowPercent = 75
	// defaultContextClearTokens 模型未声明上下文窗口时使用的清理阈值。
	defaultContextClearTokens = 32000
	// clearRetentionRounds 清理时完整保留的最近工具调用轮数。
	clearRetentionRounds = 2
)

// newContextReductionHandlers 创建大工具结果转存和上下文清理中间件，转存内容存活到本次运行结束。
func newContextReductionHandlers(ctx context.Context, model ModelConfig) ([]adk.ChatModelAgentMiddleware, error) {
	backend := filesystem.NewInMemoryBackend()
	disabled := &fsmiddleware.ToolConfig{Disable: true}
	description := "读取本次运行中因结果过大而转存的工具输出，file_path 使用转存提示中给出的路径。"
	readTool, err := fsmiddleware.New(ctx, &fsmiddleware.MiddlewareConfig{
		Backend:             backend,
		ReadFileToolConfig:  &fsmiddleware.ToolConfig{Name: offloadedResultToolName, Desc: &description},
		LsToolConfig:        disabled,
		WriteFileToolConfig: disabled,
		EditFileToolConfig:  disabled,
		GlobToolConfig:      disabled,
		GrepToolConfig:      disabled,
	})
	if err != nil {
		return nil, fmt.Errorf("create offloaded tool result read middleware: %w", err)
	}
	clearTokens := int64(defaultContextClearTokens)
	if model.ContextWindow > 0 {
		clearTokens = int64(model.ContextWindow) * contextClearWindowPercent / 100
	}
	reduce, err := reduction.New(ctx, &reduction.Config{
		Backend:          backend,
		ReadFileToolName: offloadedResultToolName,
		// 读回的内容本身不再转存或清理，避免读回后又被转走。
		TruncExcludeTools:         []string{offloadedResultToolName},
		ClearExcludeTools:         []string{offloadedResultToolName},
		MaxLengthForTrunc:         toolResultOffloadBytes,
		MaxTokensForClear:         clearTokens,
		ClearRetentionSuffixLimit: clearRetentionRounds,
		TokenCounter:              countContextTokens,
		GenTruncOffloadFilePath: func(ctx context.Context, detail *reduction.ToolDetail) (string, error) {
			path := "/trunc/" + detail.ToolContext.CallID
			slog.Warn("Agent 工具结果过大，已转存并保留预览",
				"agent_run_id", runIDFromContext(ctx), "tool_name", detail.ToolContext.Name,
				"tool_call_id", detail.ToolContext.CallID, "file_path", path)
			return path, nil
		},
		ClearPostProcess: func(ctx context.Context, state *adk.ChatModelAgentState) context.Context {
			tokens, _ := countContextTokens(ctx, state.Messages, state.ToolInfos)
			slog.Info("Agent 上下文已清理较早的工具调用",
				"agent_run_id", runIDFromContext(ctx), "clear_threshold_tokens", clearTokens, "tokens_after_clear", tokens)
			return ctx
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create context reduction middleware: %w", err)
	}
	return []adk.ChatModelAgentMiddleware{readTool, reduce}, nil
}

// countContextTokens 估算上下文 Token 数，中日韩字符按一个 Token 计，其余字符按四分之一计。
// 工具定义按参数 schema 的 JSON 计入，工具较多时其体积与消息同样占用窗口。
func countContextTokens(_ context.Context, messages []*schema.Message, tools []*schema.ToolInfo) (int64, error) {
	texts := make([]string, 0, len(messages)*2+len(tools)*3)
	for _, message := range messages {
		texts = append(texts, message.Content, message.ReasoningContent)
		for _, call := range message.ToolCalls {
			texts = append(texts, call.Function.Name, call.Function.Arguments)
		}
	}
	for _, info := range tools {
		texts = append(texts, info.Name, info.Desc)
		parameters, err := info.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return 0, fmt.Errorf("read tool %q parameters: %w", info.Name, err)
		}
		encoded, err := json.Marshal(parameters)
		if err != nil {
			return 0, fmt.Errorf("encode tool %q parameters: %w", info.Name, err)
		}
		texts = append(texts, string(encoded))
	}
	wide, narrow := 0, 0
	for _, text := range texts {
		for _, char := range text {
			if unicode.Is(unicode.Han, char) || unicode.Is(unicode.Hiragana, char) ||
				unicode.Is(unicode.Katakana, char) || unicode.Is(unicode.Hangul, char) {
				wide++
				continue
			}
			narrow++
		}
	}
	return int64(wide + narrow/4), nil
}
