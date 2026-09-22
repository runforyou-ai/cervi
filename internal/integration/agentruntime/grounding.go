package agentruntime

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
)

const (
	// truncOffloadDir 与 clearOffloadDir 是工具结果转存路径前缀，路径末段为来源工具调用编号。
	truncOffloadDir = "/trunc/"
	clearOffloadDir = "/clear/"
)

// evidenceJudge 判断一次工具调用的原始结果是否构成回答依据。
type evidenceJudge func(output string) bool

// groundingGate 按模型实际可见的上下文判定直接输出的正文是否在当前输入边界内取得有效依据；依据来源由工具名到判定函数的登记决定。
type groundingGate struct {
	adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]
	judges map[string]evidenceJudge

	mu       sync.Mutex
	valid    map[string]string   // 原始结果通过判定的工具调用编号及其原始结果。
	boundary map[string]struct{} // 当前输入边界之前已存在的工具调用编号。
	grounded bool                // 最近一次不含工具调用的模型输出是否取得依据。
}

// newGroundingGate 按依据来源登记创建一次执行尝试共用的依据门禁。
func newGroundingGate(judges map[string]evidenceJudge) *groundingGate {
	return &groundingGate{judges: judges, valid: make(map[string]string), boundary: make(map[string]struct{})}
}

// WrapInvokableToolCall 在依据来源工具返回时按原始结果判定并登记，门禁位于上下文治理内层，取得的是截断前的结果。
func (g *groundingGate) WrapInvokableToolCall(_ context.Context, endpoint adk.InvokableToolCallEndpoint, tCtx *adk.ToolContext) (adk.InvokableToolCallEndpoint, error) {
	judge, ok := g.judges[tCtx.Name]
	if !ok {
		return endpoint, nil
	}
	return func(ctx context.Context, arguments string, opts ...tool.Option) (string, error) {
		output, err := endpoint(ctx, arguments, opts...)
		if err == nil && judge(output) {
			g.mu.Lock()
			g.valid[tCtx.CallID] = output
			g.mu.Unlock()
		}
		return output, err
	}, nil
}

// resetBoundary 在认领新的持久输入时重建依据边界，依据只计入边界之后产生的工具结果。
func (g *groundingGate) resetBoundary(history []*schema.AgenticMessage) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.boundary = make(map[string]struct{})
	for _, message := range history {
		for _, block := range message.ContentBlocks {
			if block.Type == schema.ContentBlockTypeFunctionToolResult {
				g.boundary[block.FunctionToolResult.CallID] = struct{}{}
			}
		}
	}
	g.grounded = false
}

// AfterModelRewriteState 在模型给出不含工具调用的输出时，按本次发给模型的上下文判定是否取得依据。
func (g *groundingGate) AfterModelRewriteState(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.AgenticMessage], _ *adk.TypedModelContext[*schema.AgenticMessage]) (context.Context, *adk.TypedChatModelAgentState[*schema.AgenticMessage], error) {
	visible := state.Messages[:len(state.Messages)-1]
	if hasToolCalls(state.Messages[len(state.Messages)-1]) {
		return ctx, state, nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	// 汇总工具调用的名称与参数，转存读回按参数中的路径关联来源调用。
	calls := make(map[string]*schema.FunctionToolCall)
	for _, message := range visible {
		for _, block := range message.ContentBlocks {
			if block.Type == schema.ContentBlockTypeFunctionToolCall {
				calls[block.FunctionToolCall.CallID] = block.FunctionToolCall
			}
		}
	}
	g.grounded = false
	for _, message := range visible {
		for _, block := range message.ContentBlocks {
			if block.Type != schema.ContentBlockTypeFunctionToolResult {
				continue
			}
			result := block.FunctionToolResult
			call, called := calls[result.CallID]
			if _, before := g.boundary[result.CallID]; before || !called {
				continue
			}
			// 拼接工具结果中的文本块。
			var builder strings.Builder
			for _, content := range result.Content {
				if content.Type == schema.FunctionToolResultContentBlockTypeText {
					builder.WriteString(content.Text.Text)
				}
			}
			text := builder.String()
			// 依据来源的结果与原始结果一致时模型可见；截断或清理后需经转存读回。
			if call.Name == offloadedResultToolName {
				g.grounded = g.readsBackEvidence(call.Arguments, text)
			} else if output, valid := g.valid[result.CallID]; valid {
				g.grounded = text == output
			}
			if g.grounded {
				return ctx, state, nil
			}
		}
	}
	return ctx, state, nil
}

// readsBackEvidence 判断一次转存读回是否把当前边界内已通过判定的来源结果完整取回；读回文本去掉行号后须包含原始结果。
func (g *groundingGate) readsBackEvidence(arguments, text string) bool {
	input := struct {
		FilePath string `json:"file_path"`
	}{}
	if json.Unmarshal([]byte(arguments), &input) != nil {
		return false
	}
	source, trunc := strings.CutPrefix(input.FilePath, truncOffloadDir)
	if !trunc {
		var clear bool
		if source, clear = strings.CutPrefix(input.FilePath, clearOffloadDir); !clear {
			return false
		}
	}
	output, valid := g.valid[source]
	if _, before := g.boundary[source]; before || !valid {
		return false
	}
	// 读回工具按「行号、制表符、原文」逐行返回；失败说明不带行号，按未读回处理。
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		number, content, numbered := strings.Cut(line, "\t")
		if _, err := strconv.Atoi(strings.TrimSpace(number)); !numbered || err != nil {
			return false
		}
		lines[i] = content
	}
	return strings.Contains(strings.Join(lines, "\n"), output)
}

// verdict 返回最近一次直接输出正文时的依据判定。
func (g *groundingGate) verdict() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.grounded
}

// knowledgeEvidence 判定知识检索结果含有正文非空的命中记录。
func knowledgeEvidence(output string) bool {
	result := knowledgeretrieval.Result{}
	if json.Unmarshal([]byte(output), &result) != nil {
		return false
	}
	for _, record := range result.Records {
		if record.Matched && strings.TrimSpace(record.Content) != "" {
			return true
		}
	}
	return false
}
