//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// User 表示 PostgreSQL 中的用户账号。
type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID                          string    `bun:"id,pk"`
	IdentityID                  string    `bun:"identity_id"`
	OrganizationID              string    `bun:"organization_id"`
	Email                       string    `bun:"email"`
	PasswordHash                string    `bun:"password_hash"`
	RoleID                      string    `bun:"role_id"`
	Status                      string    `bun:"status"`
	Locale                      string    `bun:"locale"`
	TimeZone                    string    `bun:"time_zone"`
	MessageNotificationsEnabled bool      `bun:"message_notifications_enabled"`
	WorkspaceTabsEnabled        bool      `bun:"workspace_tabs_enabled"`
	CreatedAt                   time.Time `bun:"created_at"`
	UpdatedAt                   time.Time `bun:"updated_at"`
}
