package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// UserStatus 表示用户账号或 AI 员工的账号状态。
type UserStatus string

const (
	UserStatusActive   UserStatus = UserStatus(domain.UserStatusActive)
	UserStatusInactive UserStatus = UserStatus(domain.UserStatusInactive)
)

// WorkStatus 表示企业身份主动设置的工作状态。
type WorkStatus string

const (
	WorkStatusWorking WorkStatus = WorkStatus(domain.WorkStatusWorking)
	WorkStatusAway    WorkStatus = WorkStatus(domain.WorkStatusAway)
	WorkStatusOffDuty WorkStatus = WorkStatus(domain.WorkStatusOffDuty)
)

// CurrentUser 定义当前登录用户信息。
type CurrentUser struct {
	ID                          string     `json:"id"`
	IdentityID                  string     `json:"identityId"`
	OrganizationID              string     `json:"organizationId"`
	Email                       string     `json:"email"`
	DisplayName                 string     `json:"displayName"`
	RoleID                      string     `json:"roleId"`
	Status                      UserStatus `json:"status"`
	Locale                      Locale     `json:"locale"`
	TimeZone                    string     `json:"timeZone"`
	MessageNotificationsEnabled bool       `json:"messageNotificationsEnabled"`
	WorkStatus                  WorkStatus `json:"workStatus"`
	AvatarURL                   string     `json:"avatarUrl"`
}

// ProfileInput 定义当前用户可编辑的个人资料字段。
type ProfileInput struct {
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	AvatarFileID string `json:"avatarFileId"`
}

// ChangePasswordInput 定义当前用户修改密码所需字段。
type ChangePasswordInput struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// UserPreferencesInput 定义当前用户的偏好设置。
type UserPreferencesInput struct {
	Locale                      Locale `json:"locale"`
	TimeZone                    string `json:"timeZone"`
	MessageNotificationsEnabled bool   `json:"messageNotificationsEnabled"`
}

// UserWorkStatusInput 定义当前用户主动设置的工作状态。
type UserWorkStatusInput struct {
	WorkStatus WorkStatus `json:"workStatus"`
}

// UserListInput 定义企业成员列表查询条件。
type UserListInput struct {
	Query    string      `json:"query" query:"query"`
	Status   *UserStatus `json:"status,omitempty" query:"status"`
	RoleID   string      `json:"roleId" query:"roleId"`
	TeamID   string      `json:"teamId" query:"teamId"`
	Page     int         `json:"page" query:"page,default=1"`
	PageSize int         `json:"pageSize" query:"pageSize,default=50"`
}

// CreateUserInput 定义新增企业成员字段。
type CreateUserInput struct {
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	RoleID      string   `json:"roleId"`
	TeamIDs     []string `json:"teamIds"`
}

// UpdateUserInput 定义企业成员可编辑字段。
type UpdateUserInput struct {
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	RoleID      string   `json:"roleId"`
	TeamIDs     []string `json:"teamIds"`
}

// UserRoleChangeInput 定义一名企业成员的目标角色。
type UserRoleChangeInput struct {
	UserID string `json:"userId"`
	RoleID string `json:"roleId"`
}

// UserRoleChangesInput 定义一次批量角色调整。
type UserRoleChangesInput struct {
	Changes []UserRoleChangeInput `json:"changes"`
}

// User 定义企业成员信息。
type User struct {
	ID          string        `json:"id"`
	IdentityID  string        `json:"identityId"`
	Email       string        `json:"email"`
	DisplayName string        `json:"displayName"`
	Role        RoleSummary   `json:"role"`
	Status      UserStatus    `json:"status"`
	WorkStatus  WorkStatus    `json:"workStatus"`
	Teams       []TeamSummary `json:"teams"`
	CreatedAt   time.Time     `json:"createdAt"`
}

// UserList 定义企业成员分页结果。
type UserList struct {
	Users []User   `json:"users"`
	Page  PageInfo `json:"page"`
}
