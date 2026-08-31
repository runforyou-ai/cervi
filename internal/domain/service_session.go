package domain

// ServiceSessionStatus 定义客服处理状态。
type ServiceSessionStatus string

const (
	ServiceSessionStatusOpen   ServiceSessionStatus = "open"
	ServiceSessionStatusClosed ServiceSessionStatus = "closed"
)
