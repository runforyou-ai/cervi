//go:build server

package models

import "github.com/uptrace/bun"

// User 表示 PostgreSQL 中的用户账号。
type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID             string `bun:"id,pk" json:"id"`
	IdentityID     string `bun:"identity_id" json:"identityId"`
	OrganizationID string `bun:"organization_id" json:"organizationId"`
	Email          string `bun:"email" json:"email"`
	PasswordHash   string `bun:"password_hash" json:"-"`
	RoleID         string `bun:"role_id" json:"roleId"`
	Status         string `bun:"status" json:"status"`
	Locale         string `bun:"locale" json:"locale"`
	TimeZone       string `bun:"time_zone" json:"timeZone"`
}
