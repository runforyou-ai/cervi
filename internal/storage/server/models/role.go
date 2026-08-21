//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Role 表示 PostgreSQL 中的企业角色。
type Role struct {
	bun.BaseModel `bun:"table:roles,alias:r"`

	ID             string    `bun:"id,pk" json:"id"`
	OrganizationID string    `bun:"organization_id" json:"organizationId"`
	Kind           string    `bun:"kind" json:"kind"`
	Name           string    `bun:"name" json:"name"`
	Description    string    `bun:"description" json:"description"`
	CreatedAt      time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt      time.Time `bun:"updated_at" json:"updatedAt"`
}

// RolePermission 表示 PostgreSQL 中的角色权限关联。
type RolePermission struct {
	bun.BaseModel `bun:"table:role_permissions,alias:rp"`

	OrganizationID string    `bun:"organization_id" json:"organizationId"`
	RoleID         string    `bun:"role_id,pk" json:"roleId"`
	Permission     string    `bun:"permission,pk" json:"permission"`
	CreatedAt      time.Time `bun:"created_at" json:"createdAt"`
}
