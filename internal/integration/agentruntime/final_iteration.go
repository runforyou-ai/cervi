//go:build server

package agentruntime

import (
	"context"
	"log/slog"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// finalIterationGuard 在结果已提交或迭代预算用尽时收敛工具，让模型基于已获得的信息结束本轮。
type finalIterationGuard struct {
	adk.BaseChatModelAgentMiddleware
	maxIterations int
	groupReply    *groupReplyTool
	iterations    int
}

// newFinalIterationGuard 按本次运行的迭代上限创建收尾守卫，groupReply 非空表示本轮以结构化群聊结果结束。
func newFinalIterationGuard(maxIterations int, groupReply *groupReplyTool) *finalIterationGuard {
	return &finalIterationGuard{maxIterations: maxIterations, groupReply: groupReply}
}

// BeforeAgent 在每轮开始时重置模型规划计数。
func (g *finalIterationGuard) BeforeAgent(ctx context.Context, runCtx *adk.ChatModelAgentContext[*schema.Message]) (context.Context, *adk.ChatModelAgentContext[*schema.Message], error) {
	g.iterations = 0
	return ctx, runCtx, nil
}

// BeforeModelRewriteState 记录模型规划次数，在收尾阶段移除模型可用的工具。
// 群内结果已提交后移除全部工具，本次规划只需结束；预算用尽时保留结束工具，让模型提交最终结果。
func (g *finalIterationGuard) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	g.iterations++
	submitted := false
	if g.groupReply != nil {
		_, submitted = g.groupReply.peek()
	}
	if !submitted && g.iterations < g.maxIterations {
		return ctx, state, nil
	}
	hint, keptTool := "本轮结果已提交，请直接结束。", ""
	if !submitted {
		hint = "工具调用次数已达本轮上限，请基于已获得的信息给出最终回答。"
		if g.groupReply != nil {
			keptTool = groupReplyToolName
			hint = "工具调用次数已达本轮上限，请基于已获得的信息立即通过 " + groupReplyToolName + " 提交最终结果。"
		}
		slog.Warn("Agent 工具调用次数达到本轮上限，进入收尾规划",
			"agent_run_id", runIDFromContext(ctx), "max_iterations", g.maxIterations)
	}
	// 空工具列表必须保持非 nil，否则框架按未设置处理并填回全量工具。
	kept := make([]*schema.ToolInfo, 0, 1)
	for _, info := range state.ToolInfos {
		if info.Name == keptTool {
			kept = append(kept, info)
		}
	}
	state.ToolInfos = kept
	state.DeferredToolInfos = nil
	state.Messages = append(state.Messages, schema.UserMessage(hint))
	return ctx, state, nil
}
