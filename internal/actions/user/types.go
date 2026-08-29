//go:build server

package user

import (
	"time"

	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ListInput 定义企业成员目录查询条件。
type ListInput struct {
	Query    string
	Status   domain.UserStatus
	RoleID   string
	TeamID   string
	Page     int
	PageSize int
}

// CreateInput 定义新增企业成员字段。
type CreateInput struct {
	DisplayName string
	Email       string
	Password    string
	RoleID      string
	TeamIDs     []string
}

// UpdateInput 定义企业成员可编辑字段。
type UpdateInput struct {
	DisplayName string
	Email       string
	RoleID      string
	TeamIDs     []string
}

// RoleChangeInput 定义一名企业成员的目标角色。
type RoleChangeInput struct {
	UserID string
	RoleID string
}

// ProfileInput 定义当前用户可编辑的个人资料字段。
type ProfileInput struct {
	DisplayName  string
	Email        string
	AvatarFileID string
}

// ChangePasswordInput 定义当前用户修改密码所需字段。
type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

// PreferencesInput 定义当前用户的偏好设置。
type PreferencesInput struct {
	Locale                      domain.Locale
	TimeZone                    string
	MessageNotificationsEnabled bool
	WorkspaceTabsEnabled        bool
}

// WorkStatusInput 定义当前用户主动设置的工作状态。
type WorkStatusInput struct {
	WorkStatus domain.WorkStatus
}

// User 定义企业成员信息。
type User struct {
	ID          string `bun:"id"`
	IdentityID  string `bun:"identity_id"`
	Email       string
	DisplayName string
	RoleID      string          `bun:"role_id"`
	RoleKind    domain.RoleKind `bun:"role_kind"`
	RoleName    string          `bun:"role_name"`
	Status      domain.UserStatus
	WorkStatus  domain.WorkStatus
	Teams       []TeamSummary
	CreatedAt   time.Time
}

// TeamSummary 定义成员所属团队的精简字段。
type TeamSummary = teamaction.Summary

// ListOutput 定义企业成员分页结果。
type ListOutput struct {
	Users []User
	Page  common.PageInfo
}
