package domain

// ServiceSessionStatus 定义客服处理状态。
type ServiceSessionStatus string

const (
	ServiceSessionStatusOpen   ServiceSessionStatus = "open"
	ServiceSessionStatusClosed ServiceSessionStatus = "closed"
)

// ServiceAudience 定义服务对象：customer 为外部客户，employee 为本企业员工，partner 为伙伴。
type ServiceAudience string

const (
	ServiceAudienceCustomer ServiceAudience = "customer"
	ServiceAudienceEmployee ServiceAudience = "employee"
	ServiceAudiencePartner  ServiceAudience = "partner"
)

// ServiceSessionCloseReason 定义客服处理周期的结束方式。
type ServiceSessionCloseReason string

const (
	// ServiceSessionCloseAIResolved 表示客户确认问题已解决后由 AI 负责人关闭。
	ServiceSessionCloseAIResolved ServiceSessionCloseReason = "ai_resolved"
	// ServiceSessionCloseCustomerUnresponsive 表示 AI 跟进或请求确认后客户超时未回复而关闭。
	ServiceSessionCloseCustomerUnresponsive ServiceSessionCloseReason = "customer_unresponsive"
	// ServiceSessionCloseManual 表示由客服人工关闭。
	ServiceSessionCloseManual ServiceSessionCloseReason = "manual"
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
