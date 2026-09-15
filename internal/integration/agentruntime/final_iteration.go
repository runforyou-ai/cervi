//go:build server

package agentruntime

import (
	"context"
	"log/slog"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// finalIterationGuard 在迭代预算用尽时收敛工具，让模型基于已获得的信息结束本轮。
type finalIterationGuard struct {
	adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]
	maxIterations int
	iterations    int
}

// newFinalIterationGuard 按本次运行的迭代上限创建收尾守卫。
func newFinalIterationGuard(maxIterations int) *finalIterationGuard {
	return &finalIterationGuard{maxIterations: maxIterations}
}

// BeforeAgent 在每轮开始时重置模型规划计数。
func (g *finalIterationGuard) BeforeAgent(ctx context.Context, runCtx *adk.ChatModelAgentContext[*schema.AgenticMessage]) (context.Context, *adk.ChatModelAgentContext[*schema.AgenticMessage], error) {
	g.iterations = 0
	return ctx, runCtx, nil
}

// BeforeModelRewriteState 记录模型规划次数，预算用尽时移除全部工具并要求模型给出最终回答。
func (g *finalIterationGuard) BeforeModelRewriteState(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.AgenticMessage], _ *adk.TypedModelContext[*schema.AgenticMessage]) (context.Context, *adk.TypedChatModelAgentState[*schema.AgenticMessage], error) {
	g.iterations++
	if g.iterations < g.maxIterations {
		return ctx, state, nil
	}
	slog.Warn("Agent 工具调用次数达到本轮上限，进入收尾规划",
		"agent_run_id", runIDFromContext(ctx), "max_iterations", g.maxIterations)
	// 空工具列表必须保持非 nil，否则框架按未设置处理并填回全量工具。
	state.ToolInfos = []*schema.ToolInfo{}
	state.DeferredToolInfos = nil
	state.Messages = append(state.Messages, schema.UserAgenticMessage("工具调用次数已达本轮上限，请基于已获得的信息给出最终回答。"))
	return ctx, state, nil
}
