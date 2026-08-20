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
