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
	AgentInputKindAgentDirect  AgentInputKind = "agent_direct"
	AgentInputKindCustomerAuto AgentInputKind = "customer_auto"
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
