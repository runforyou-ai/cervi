//go:build server

package models

// Identity 表示当前用户账号、企业身份、所属企业及本次请求使用的登录令牌。
type Identity struct {
	Organization         Organization
	OrganizationIdentity OrganizationIdentity
	User                 User
	Token                Token
}
