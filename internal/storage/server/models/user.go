//go:build server

package models

import "github.com/uptrace/bun"

// User 表示 PostgreSQL 中的用户账号子实体。
type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID             string  `bun:"id,pk" json:"id"`
	OrganizationID string  `bun:"organization_id" json:"organizationId"`
	Email          string  `bun:"email" json:"email"`
	PasswordHash   string  `bun:"password_hash" json:"-"`
	RoleID         string  `bun:"role_id" json:"roleId"`
	Locale         string  `bun:"locale" json:"locale"`
	TimeZone       string  `bun:"time_zone" json:"timeZone"`
	WorkStatus     string  `bun:"work_status" json:"workStatus"`
	DisplayName    string  `bun:"display_name" json:"displayName"`
	Status         string  `bun:"status" json:"status"`
	AvatarFileID   *string `bun:"avatar_file_id" json:"avatarFileId"`
}
