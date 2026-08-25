package domain

// ServiceSessionStatus 定义客服处理状态。
type ServiceSessionStatus string

const (
	ServiceSessionStatusWaiting ServiceSessionStatus = "waiting"
	ServiceSessionStatusActive  ServiceSessionStatus = "active"
	ServiceSessionStatusPending ServiceSessionStatus = "pending"
	ServiceSessionStatusClosed  ServiceSessionStatus = "closed"
)
