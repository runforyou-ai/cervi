package domain

// MemberIdentityType 定义企业成员类型。
type MemberIdentityType string

const (
	MemberIdentityTypeUser  MemberIdentityType = "user"
	MemberIdentityTypeAgent MemberIdentityType = "agent"
)
