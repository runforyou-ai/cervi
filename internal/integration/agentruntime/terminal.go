//go:build server

package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const (
	askCustomerToolName = "ask_customer"
	handoffToolName     = "handoff_to_human"
	// correctionLimit 是一次执行尝试内允许纠正无效终止输出的次数。
	correctionLimit = 1
	// handoffReasonTextMaxRunes 限制转交原因写入系统事件的长度。
	handoffReasonTextMaxRunes = 500
)

// TerminalDecision 定义一次执行的结束方式；Kind 为空表示直接输出正文作为回答。
type TerminalDecision struct {
	Kind       domain.AgentRunOutcome
	Purpose    domain.AgentAskCustomerPurpose // ask_customer 的发问用途。
	Reason     domain.AgentHandoffReason      // handoff 的原因。
	ReasonText string                         // handoff 时模型写明的转交原因，仅成员可见。
}

// Outcome 返回决定对应的运行结果类型。
func (d TerminalDecision) Outcome() domain.AgentRunOutcome {
	if d.Kind == "" {
		return domain.AgentRunOutcomeReply
	}
	return d.Kind
}

type askCustomerInput struct {
	Purpose domain.AgentAskCustomerPurpose `json:"purpose"`
	Message string                         `json:"message"`
}

type handoffInput struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// terminalIntent 记录一次校验通过的终止工具调用。
type terminalIntent struct {
	decision TerminalDecision
	message  string
}

// terminalTools 注册客服场景的终止工具，校验同批调用与参数，并登记终止意图；工具执行只记录意图，不产生外部副作用。
type terminalTools struct {
	adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]
	mu          sync.Mutex
	batchIssue  string // 最近一次模型输出的同批违规说明，空表示合法。
	batchSeen   bool   // 当前批次的违规已计入纠正额度。
	corrections int
	intents     map[string]terminalIntent
	forced      *terminalIntent // 纠正额度用尽后由 Runtime 构造的转人工。
	handoff     bool            // 本次执行已固定转人工决定。
}

// newTerminalTools 创建一次执行尝试共用的终止工具状态。
func newTerminalTools() *terminalTools {
	return &terminalTools{intents: make(map[string]terminalIntent)}
}

// tools 返回 ask_customer 与 handoff_to_human 两个终止工具。
func (t *terminalTools) tools() []tool.BaseTool {
	return []tool.BaseTool{
		&terminalTool{info: &schema.ToolInfo{
			Name: askCustomerToolName,
			Desc: "向客户发送追问、确认或问候并等待客户回复。message 是发给客户的完整内容。",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"purpose": {Type: schema.String, Required: true, Desc: "greeting 问候，clarify 请客户补充信息，confirm 请客户确认", Enum: []string{
					string(domain.AgentAskCustomerPurposeGreeting), string(domain.AgentAskCustomerPurposeClarify), string(domain.AgentAskCustomerPurposeConfirm),
				}},
				"message": {Type: schema.String, Required: true, Desc: "发给客户的内容"},
			}),
		}, run: t.askCustomer},
		&terminalTool{info: &schema.ToolInfo{
			Name: handoffToolName,
			Desc: "把当前客户会话交给人工客服。reason 写明转交原因，仅企业成员可见；message 是发给客户的说明，可以为空。",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"reason":  {Type: schema.String, Required: true, Desc: "转交原因"},
				"message": {Type: schema.String, Desc: "发给客户的说明"},
			}),
		}, run: t.handoffToHuman},
	}
}

// askCustomer 校验追问参数并登记 ask_customer 意图。
func (t *terminalTools) askCustomer(ctx context.Context, arguments string) error {
	input := askCustomerInput{}
	if err := json.Unmarshal([]byte(arguments), &input); err != nil {
		return fmt.Errorf("参数不是有效的 JSON：%w", err)
	}
	input.Message = strings.TrimSpace(input.Message)
	switch input.Purpose {
	case domain.AgentAskCustomerPurposeGreeting, domain.AgentAskCustomerPurposeClarify, domain.AgentAskCustomerPurposeConfirm:
	default:
		return errors.New("purpose 只能是 greeting、clarify 或 confirm")
	}
	if input.Message == "" {
		return errors.New("message 不能为空")
	}
	t.record(ctx, terminalIntent{decision: TerminalDecision{Kind: domain.AgentRunOutcomeAskCustomer, Purpose: input.Purpose}, message: input.Message})
	return nil
}

// handoffToHuman 校验转交参数并登记不可降级的 handoff 意图。
func (t *terminalTools) handoffToHuman(ctx context.Context, arguments string) error {
	input := handoffInput{}
	if err := json.Unmarshal([]byte(arguments), &input); err != nil {
		return fmt.Errorf("参数不是有效的 JSON：%w", err)
	}
	reason := []rune(strings.TrimSpace(input.Reason))
	if len(reason) == 0 {
		return errors.New("reason 不能为空")
	}
	if len(reason) > handoffReasonTextMaxRunes {
		reason = reason[:handoffReasonTextMaxRunes]
	}
	t.record(ctx, terminalIntent{decision: TerminalDecision{
		Kind: domain.AgentRunOutcomeHandoff, Reason: domain.AgentHandoffReasonModelRequested, ReasonText: string(reason),
	}, message: strings.TrimSpace(input.Message)})
	return nil
}

// record 按工具调用编号登记终止意图，并请求本次模型规划在工具结果后直接结束。
func (t *terminalTools) record(ctx context.Context, intent terminalIntent) {
	t.mu.Lock()
	t.intents[compose.GetToolCallID(ctx)] = intent
	if intent.decision.Kind == domain.AgentRunOutcomeHandoff {
		t.handoff = true
	}
	t.mu.Unlock()
	if err := adk.SetToolReturnDirectly(ctx); err != nil {
		slog.Warn("终止工具请求直接返回失败", "agent_run_id", runIDFromContext(ctx), "error", err)
	}
}

// AfterModelRewriteState 在工具执行前检查本次模型输出：终止工具至多一个且不与其他工具同批调用。
func (t *terminalTools) AfterModelRewriteState(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.AgenticMessage], _ *adk.TypedModelContext[*schema.AgenticMessage]) (context.Context, *adk.TypedChatModelAgentState[*schema.AgenticMessage], error) {
	message := state.Messages[len(state.Messages)-1]
	names := make([]string, 0)
	terminal := 0
	for _, block := range message.ContentBlocks {
		if block.Type != schema.ContentBlockTypeFunctionToolCall {
			continue
		}
		names = append(names, block.FunctionToolCall.Name)
		if block.FunctionToolCall.Name == askCustomerToolName || block.FunctionToolCall.Name == handoffToolName {
			terminal++
		}
	}
	issue := ""
	switch {
	case terminal > 1:
		issue = "ask_customer 与 handoff_to_human 一次只能调用其中一个"
	case terminal == 1 && len(names) > 1:
		issue = "ask_customer 或 handoff_to_human 必须单独调用，不能与其他工具同时调用"
	}
	t.mu.Lock()
	t.batchIssue, t.batchSeen = issue, false
	t.mu.Unlock()
	return ctx, state, nil
}

// middleware 拦截同批违规的工具调用并处理终止工具参数错误：纠正额度内把错误交回模型，额度用尽时构造转人工并直接结束。
func (t *terminalTools) middleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				t.mu.Lock()
				issue, counted := t.batchIssue, t.batchSeen
				t.batchSeen = t.batchSeen || issue != ""
				t.mu.Unlock()
				if issue != "" {
					// 同一批次只计一次纠正，其余调用直接返回同样的说明。
					if counted {
						return nil, errors.New(issue)
					}
					return nil, t.reject(ctx, issue)
				}
				output, err := next(ctx, input)
				if err == nil || (input.Name != askCustomerToolName && input.Name != handoffToolName) || ctx.Err() != nil {
					return output, err
				}
				return nil, t.reject(ctx, err.Error())
			}
		},
	}
}

// reject 消耗一次纠正额度并返回交给模型的错误；额度用尽时登记 Runtime 构造的转人工并请求直接结束。
func (t *terminalTools) reject(ctx context.Context, issue string) error {
	t.mu.Lock()
	if t.corrections < correctionLimit {
		t.corrections++
		t.mu.Unlock()
		slog.Warn("Agent 终止输出无效，要求模型纠正", "agent_run_id", runIDFromContext(ctx), "issue", issue)
		return fmt.Errorf("%s。需要客户补充信息或只是问候请单独调用 ask_customer；无法解答请单独调用 handoff_to_human", issue)
	}
	if t.forced == nil {
		t.forced = &terminalIntent{decision: TerminalDecision{Kind: domain.AgentRunOutcomeHandoff, Reason: domain.AgentHandoffReasonInvalidOutput}}
		t.handoff = true
	}
	t.mu.Unlock()
	slog.Warn("Agent 纠正后仍输出无效终止调用，转交人工", "agent_run_id", runIDFromContext(ctx), "issue", issue)
	if err := adk.SetToolReturnDirectly(ctx); err != nil {
		slog.Warn("终止工具请求直接返回失败", "agent_run_id", runIDFromContext(ctx), "error", err)
	}
	return errors.New(issue)
}

// beginTurn 在认领新输入时清空上一轮的非转人工意图。
func (t *terminalTools) beginTurn() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.handoff {
		t.intents = make(map[string]terminalIntent)
	}
}

// handoffFixed 判断本次执行是否已固定转人工决定。
func (t *terminalTools) handoffFixed() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.handoff
}

// decision 按本轮成功的终止工具调用编号取得终止意图；已固定的转人工优先。
func (t *terminalTools) decision(resultCallIDs []string) (terminalIntent, bool) {
	if t == nil {
		return terminalIntent{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.forced != nil {
		return *t.forced, true
	}
	var found *terminalIntent
	for _, intent := range t.intents {
		if intent.decision.Kind == domain.AgentRunOutcomeHandoff {
			return intent, true
		}
	}
	for _, callID := range resultCallIDs {
		if intent, ok := t.intents[callID]; ok {
			found = &intent
		}
	}
	if found == nil {
		return terminalIntent{}, false
	}
	return *found, true
}

// terminalTool 以原始参数执行终止工具，参数错误交由中间件计入纠正额度。
type terminalTool struct {
	info *schema.ToolInfo
	run  func(context.Context, string) error
}

// Info 返回工具定义。
func (t *terminalTool) Info(context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

// InvokableRun 校验参数并登记意图，返回给过程记录的受理结果。
func (t *terminalTool) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	if err := t.run(ctx, arguments); err != nil {
		return "", err
	}
	return `{"accepted":true}`, nil
}
