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
	// ServiceAttentionAssigned 提醒成员处理新分配给本人的周期。
	ServiceAttentionAssigned ServiceAttentionReason = "assigned"
	// ServiceAttentionResponseOverdue 提醒负责人客户等待回复已超时。
	ServiceAttentionResponseOverdue ServiceAttentionReason = "response_overdue"
	// ServiceAttentionQueueWaiting 提醒队列对应的客服有周期等待超时。
	ServiceAttentionQueueWaiting ServiceAttentionReason = "queue_waiting"
	// ServiceAttentionReturned 告知原负责人其超时未回复的周期已退回队列。
	ServiceAttentionReturned ServiceAttentionReason = "returned"
)
