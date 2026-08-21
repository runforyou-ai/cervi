package domain

// MemberIdentityType 定义可以加入团队的企业内部身份类型。
type MemberIdentityType string

const (
	MemberIdentityTypeUser  MemberIdentityType = "user"
	MemberIdentityTypeAgent MemberIdentityType = "agent"
)
