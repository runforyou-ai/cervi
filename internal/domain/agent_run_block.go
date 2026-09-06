package domain

// AgentRunBlockKind 定义 Agent 中间内容的语义类型。
type AgentRunBlockKind string

const (
	AgentRunBlockThinking AgentRunBlockKind = "thinking"
	AgentRunBlockContent  AgentRunBlockKind = "content"
	AgentRunBlockToolCall AgentRunBlockKind = "tool_call"
)

// AgentToolCallStatus 定义一次工具调用的执行状态。
type AgentToolCallStatus string

const (
	AgentToolCallQueued    AgentToolCallStatus = "queued"
	AgentToolCallRunning   AgentToolCallStatus = "running"
	AgentToolCallSucceeded AgentToolCallStatus = "succeeded"
	AgentToolCallFailed    AgentToolCallStatus = "failed"
)
