package domain

// OrganizationIdentityType 定义企业身份类型。
type OrganizationIdentityType string

const (
	OrganizationIdentityTypeUser  OrganizationIdentityType = "user"
	OrganizationIdentityTypeAgent OrganizationIdentityType = "agent"
)
