package domain

// UserStatus 定义用户账号或 AI 员工的账号状态。
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
)

// WorkStatus 定义企业身份主动设置的工作状态。
type WorkStatus string

const (
	WorkStatusWorking WorkStatus = "working"
	WorkStatusAway    WorkStatus = "away"
	WorkStatusOffDuty WorkStatus = "off_duty"
)
