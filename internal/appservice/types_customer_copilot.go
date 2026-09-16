package appservice

import "time"

// CustomerCopilotThread 定义客户会话中 Copilot 线程的摘要。
type CustomerCopilotThread struct {
	ID                  string    `json:"id"`
	Title               string    `json:"title"`
	AgentIdentityID     string    `json:"agentIdentityId"`
	AgentName           string    `json:"agentName"`
	AgentAvatarURL      string    `json:"agentAvatarUrl"`
	AgentActive         bool      `json:"agentActive"`
	CreatedByIdentityID string    `json:"createdByIdentityId"`
	CreatedByName       string    `json:"createdByName"`
	CreatedAt           time.Time `json:"createdAt"`
	LastActivityAt      time.Time `json:"lastActivityAt"`
}

// CustomerCopilotThreadList 定义客户会话按最近活动倒序排列的 Copilot 线程。
type CustomerCopilotThreadList struct {
	Threads []CustomerCopilotThread `json:"threads"`
}

// FirstCustomerCopilotMessageInput 定义新线程的稳定编号、回答的 AI 员工和首条提问。
type FirstCustomerCopilotMessageInput struct {
	ThreadID        string `json:"threadId"`
	AgentIdentityID string `json:"agentIdentityId"`
	ClientMessageID string `json:"clientMessageId"`
	Body            string `json:"body"`
}

// FirstCustomerCopilotMessageResult 定义首条提问确认的线程和消息。
type FirstCustomerCopilotMessageResult struct {
	Thread  CustomerCopilotThread `json:"thread"`
	Message ConversationMessage   `json:"message"`
}

// CustomerCopilotTextMessageInput 定义发给 Copilot 线程的成员提问。
type CustomerCopilotTextMessageInput struct {
	ClientMessageID  string `json:"clientMessageId"`
	Body             string `json:"body"`
	ReplyToMessageID string `json:"replyToMessageId"`
}
