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

// AgentTriggerType 定义 Agent 运行的业务触发入口。
type AgentTriggerType string

const (
	AgentTriggerTypeDirect       AgentTriggerType = "agent_direct"
	AgentTriggerTypeCustomerAuto AgentTriggerType = "customer_auto"
)

// AgentRunErrorCode 定义 Agent 运行取消或失败的稳定原因。
type AgentRunErrorCode string

const (
	AgentRunErrorCodeAssigneeChanged AgentRunErrorCode = "assignee_changed"
	AgentRunErrorCodeSessionClosed   AgentRunErrorCode = "session_closed"
)
