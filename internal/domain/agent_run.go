package domain

// AgentRunStatus 定义一次 Agent 业务运行的状态。
type AgentRunStatus string

const (
	AgentRunStatusQueued    AgentRunStatus = "queued"
	AgentRunStatusRunning   AgentRunStatus = "running"
	AgentRunStatusSucceeded AgentRunStatus = "succeeded"
	AgentRunStatusFailed    AgentRunStatus = "failed"
	AgentRunStatusCancelled AgentRunStatus = "cancelled"
)

// AgentExecutionScopeKind 定义 Agent 执行范围的类型。
type AgentExecutionScopeKind string

const (
	AgentExecutionScopeConversation   AgentExecutionScopeKind = "conversation"
	AgentExecutionScopeServiceSession AgentExecutionScopeKind = "service_session"
)

// AgentInputKind 定义 Agent 持久输入的业务入口。
type AgentInputKind string

const (
	AgentInputKindMention      AgentInputKind = "mention"
	AgentInputKindHandoff      AgentInputKind = "handoff"
	AgentInputKindAgentDirect  AgentInputKind = "agent_direct"
	AgentInputKindCustomerAuto AgentInputKind = "customer_auto"
	AgentInputKindCopilot      AgentInputKind = "copilot"
)

// AgentRunErrorCode 定义 Agent 运行取消或失败的稳定原因。
type AgentRunErrorCode string

const (
	AgentRunErrorCodeAssigneeChanged AgentRunErrorCode = "assignee_changed"
	AgentRunErrorCodeSessionClosed   AgentRunErrorCode = "session_closed"
	AgentRunErrorCodeUserCancelled   AgentRunErrorCode = "user_cancelled"
	AgentRunErrorCodeBotChanged      AgentRunErrorCode = "bot_changed"
	AgentRunErrorCodeAgentRemoved    AgentRunErrorCode = "agent_removed"
)

// AgentRunErrorCodeAgentUnavailable 表示 AI 员工被停用或失去接客资格，由管理操作取消运行。
const AgentRunErrorCodeAgentUnavailable AgentRunErrorCode = "agent_unavailable"

// AgentRunOutcome 定义一次 Agent 运行的结束方式。
type AgentRunOutcome string

const (
	AgentRunOutcomeReply       AgentRunOutcome = "reply"
	AgentRunOutcomeAskCustomer AgentRunOutcome = "ask_customer"
	AgentRunOutcomeHandoff     AgentRunOutcome = "handoff"
)

// AgentHandoffReason 定义 AI 客服把会话转交人工的原因。
type AgentHandoffReason string

const (
	AgentHandoffReasonModelRequested       AgentHandoffReason = "model_requested"
	AgentHandoffReasonInsufficientEvidence AgentHandoffReason = "insufficient_evidence"
	AgentHandoffReasonBudgetExhausted      AgentHandoffReason = "budget_exhausted"
	AgentHandoffReasonInvalidOutput        AgentHandoffReason = "invalid_output"
	AgentHandoffReasonRuntimeFailed        AgentHandoffReason = "runtime_failed"
	AgentHandoffReasonTimeout              AgentHandoffReason = "timeout"
	AgentHandoffReasonAgentUnavailable     AgentHandoffReason = "agent_unavailable"
)

// AgentAskCustomerPurpose 定义 AI 客服向客户发问的用途，只用于审计与统计。
type AgentAskCustomerPurpose string

const (
	AgentAskCustomerPurposeGreeting AgentAskCustomerPurpose = "greeting"
	AgentAskCustomerPurposeClarify  AgentAskCustomerPurpose = "clarify"
	AgentAskCustomerPurposeConfirm  AgentAskCustomerPurpose = "confirm"
)
