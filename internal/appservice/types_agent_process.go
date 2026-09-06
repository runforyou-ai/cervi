package appservice

import "github.com/runforyou-ai/cervi/internal/domain"

// AgentRunBlockKind 定义思考区域中的内容类型。
type AgentRunBlockKind string

const (
	AgentRunBlockThinking AgentRunBlockKind = AgentRunBlockKind(domain.AgentRunBlockThinking)
	AgentRunBlockContent  AgentRunBlockKind = AgentRunBlockKind(domain.AgentRunBlockContent)
	AgentRunBlockToolCall AgentRunBlockKind = AgentRunBlockKind(domain.AgentRunBlockToolCall)
)

// AgentToolCallStatus 定义工具执行状态。
type AgentToolCallStatus string

const (
	AgentToolCallQueued    AgentToolCallStatus = AgentToolCallStatus(domain.AgentToolCallQueued)
	AgentToolCallRunning   AgentToolCallStatus = AgentToolCallStatus(domain.AgentToolCallRunning)
	AgentToolCallSucceeded AgentToolCallStatus = AgentToolCallStatus(domain.AgentToolCallSucceeded)
	AgentToolCallFailed    AgentToolCallStatus = AgentToolCallStatus(domain.AgentToolCallFailed)
)

// ConversationAgentProcess 定义成功消息的有序过程和模型用量。
type ConversationAgentProcess struct {
	ID                   string                 `json:"id"`
	DurationMilliseconds int64                  `json:"durationMilliseconds"`
	InputTokens          int                    `json:"inputTokens"`
	OutputTokens         int                    `json:"outputTokens"`
	Blocks               []AgentRunContentBlock `json:"blocks"`
}

// AgentRunContentBlock 定义一个独立的文本或工具内容块。
type AgentRunContentBlock struct {
	ID       string            `json:"id"`
	Position int64             `json:"position"`
	Kind     AgentRunBlockKind `json:"kind"`
	Text     string            `json:"text"`
	ToolCall *AgentToolCall    `json:"toolCall"`
}

// AgentToolCall 定义完整工具参数、结果和错误。
type AgentToolCall struct {
	Name      string              `json:"name"`
	Arguments string              `json:"arguments"`
	Result    *string             `json:"result"`
	Error     *string             `json:"error"`
	Status    AgentToolCallStatus `json:"status"`
}

// ConversationAgentRun 定义消息窗口中的最近一次运行状态。
type ConversationAgentRun struct {
	AgentName string         `json:"agentName"`
	ID        string         `json:"id"`
	Status    AgentRunStatus `json:"status"`
	ErrorCode *string        `json:"errorCode"`
	LastError *string        `json:"lastError"`
}
