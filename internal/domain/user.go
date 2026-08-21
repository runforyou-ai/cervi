package domain

// UserRole 定义企业成员角色。
type UserRole string

const (
	UserRoleOwner  UserRole = "owner"
	UserRoleMember UserRole = "member"
)

// UserStatus 定义企业成员状态。
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
)

// WorkStatus 定义成员主动设置的工作状态。
type WorkStatus string

const (
	WorkStatusWorking WorkStatus = "working"
	WorkStatusAway    WorkStatus = "away"
	WorkStatusOffDuty WorkStatus = "off_duty"
)
