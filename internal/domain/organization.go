package domain

// OrganizationNameMaxLength 是企业名称允许的最大字符数。
const OrganizationNameMaxLength = 32

// OrganizationLifecycleStatus 定义企业生命周期状态。
type OrganizationLifecycleStatus string

const (
	OrganizationLifecycleActive    OrganizationLifecycleStatus = "active"
	OrganizationLifecycleSuspended OrganizationLifecycleStatus = "suspended"
	OrganizationLifecycleDeleting  OrganizationLifecycleStatus = "deleting"
	OrganizationLifecycleDeleted   OrganizationLifecycleStatus = "deleted"
)
