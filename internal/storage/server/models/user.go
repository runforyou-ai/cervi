//go:build server

package models

import "github.com/uptrace/bun"

// User 表示 PostgreSQL 中的企业成员。
type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID             string `bun:"id,pk" json:"id"`
	OrganizationID string `bun:"organization_id" json:"organizationId"`
	Email          string `bun:"email" json:"email"`
	DisplayName    string `bun:"display_name" json:"displayName"`
	PasswordHash   string `bun:"password_hash" json:"-"`
	Role           string `bun:"role" json:"role"`
	Status         string `bun:"status" json:"status"`
}
