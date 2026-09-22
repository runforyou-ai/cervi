package domain

// ServiceSessionStatus 定义客服处理状态。
type ServiceSessionStatus string

const (
	ServiceSessionStatusOpen   ServiceSessionStatus = "open"
	ServiceSessionStatusClosed ServiceSessionStatus = "closed"
)

// DefaultMaxServiceSessions 是成员默认的最大接待量。
const DefaultMaxServiceSessions = 10

// ServiceAttentionReason 定义提醒成员处理客服处理周期的原因。
type ServiceAttentionReason string

const (
	ServiceAttentionAssigned ServiceAttentionReason = "assigned"
)
