package domain

// ConversationType 定义会话类型。
type ConversationType string

const (
	ConversationTypeDirect   ConversationType = "direct"
	ConversationTypeGroup    ConversationType = "group"
	ConversationTypeCustomer ConversationType = "customer"
)

// ConversationStatus 定义会话生命周期状态。
type ConversationStatus string

const (
	ConversationStatusActive   ConversationStatus = "active"
	ConversationStatusArchived ConversationStatus = "archived"
)
