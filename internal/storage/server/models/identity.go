//go:build server

package models

// Identity 表示当前用户账号、企业身份及所属企业。
type Identity struct {
	Organization         Organization         `json:"organization"`
	OrganizationIdentity OrganizationIdentity `json:"organizationIdentity"`
	User                 User                 `json:"user"`
}
