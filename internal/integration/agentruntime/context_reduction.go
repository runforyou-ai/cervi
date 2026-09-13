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
	// toolResultWindowPercent 单次工具结果在上下文中最多占模型窗口的百分比，超出即转存。
	toolResultWindowPercent = 10
	// contextClearWindowPercent 上下文 Token 占模型窗口达到该百分比时清理历史工具调用。
	contextClearWindowPercent = 75
	// minToolResultOffloadBytes 转存阈值下限，保证小窗口模型的首尾预览仍有可读内容。
	minToolResultOffloadBytes = 4000
	// bytesPerToken 按中日韩文本的三字节一 Token 换算，使字节上限不会超出对应的 Token 预算。
	bytesPerToken = 3
	// defaultContextWindowTokens 模型未声明上下文窗口时使用的窗口估值。
	defaultContextWindowTokens = 32000
	// clearRetentionRounds 清理时完整保留的最近工具调用轮数。
	clearRetentionRounds = 2
	// historyWindowPercent 认领的会话历史最多占模型窗口的百分比，其余为指令、工具定义和本轮工具结果预留。
	historyWindowPercent = 50
)

// contextWindowTokens 返回本次运行按模型窗口计算限额时使用的 Token 数。
func contextWindowTokens(model ModelConfig) int {
	if model.ContextWindow > 0 {
		return model.ContextWindow
	}
	return defaultContextWindowTokens
}

// newContextReductionHandlers 创建大工具结果转存和上下文清理中间件，转存内容存活到本次运行结束。
func newContextReductionHandlers(ctx context.Context, window int) ([]adk.ChatModelAgentMiddleware, error) {
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
	clearTokens := int64(window) * contextClearWindowPercent / 100
	offloadBytes := offloadThresholdBytes(window)
	reduce, err := reduction.New(ctx, &reduction.Config{
		Backend:          backend,
		ReadFileToolName: offloadedResultToolName,
		// 读回工具的结果不参与转存和清理。
		TruncExcludeTools:         []string{offloadedResultToolName},
		ClearExcludeTools:         []string{offloadedResultToolName},
		MaxLengthForTrunc:         offloadBytes,
		MaxTokensForClear:         clearTokens,
		ClearRetentionSuffixLimit: clearRetentionRounds,
		TokenCounter:              countContextTokens,
		GenTruncOffloadFilePath: func(ctx context.Context, detail *reduction.ToolDetail) (string, error) {
			path := "/trunc/" + detail.ToolContext.CallID
			slog.Warn("Agent 工具结果过大，已转存并保留预览",
				"agent_run_id", runIDFromContext(ctx), "tool_name", detail.ToolContext.Name,
				"tool_call_id", detail.ToolContext.CallID, "file_path", path, "offload_threshold_bytes", offloadBytes)
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

// offloadThresholdBytes 按模型窗口推导单次工具结果的转存阈值。
func offloadThresholdBytes(window int) int {
	return max(window*toolResultWindowPercent/100*bytesPerToken, minToolResultOffloadBytes)
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
	total := 0
	for _, text := range texts {
		total += estimateTextTokens(text)
	}
	return int64(total), nil
}

// estimateTextTokens 估算单段文本的 Token 数，中日韩字符按一个计，其余字符按四分之一计。
func estimateTextTokens(text string) int {
	wide, narrow := 0, 0
	for _, char := range text {
		if unicode.Is(unicode.Han, char) || unicode.Is(unicode.Hiragana, char) ||
			unicode.Is(unicode.Katakana, char) || unicode.Is(unicode.Hangul, char) {
			wide++
			continue
		}
		narrow++
	}
	return wide + narrow/4
}

// trimClaimedHistory 按模型窗口预算保留最近的会话历史，超出预算的较早消息不进入本次输入。
// 最新一条无论多长都保留，它是本次运行要处理的输入。
func trimClaimedHistory(ctx context.Context, messages []Message, window int) []Message {
	budget := window * historyWindowPercent / 100
	total := 0
	for i := len(messages) - 1; i >= 0; i-- {
		total += estimateTextTokens(messages[i].Content)
		if total <= budget || i == len(messages)-1 {
			continue
		}
		slog.Warn("会话历史超出模型窗口预算，只保留较新的消息",
			"agent_run_id", runIDFromContext(ctx), "budget_tokens", budget,
			"message_count", len(messages), "kept_message_count", len(messages)-i-1)
		return messages[i+1:]
	}
	return messages
}
