//go:build server

package agent

import (
	"time"

	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// CreateInput 定义新增 AI 员工字段，AvatarFileID 为空时不设置头像。
type CreateInput struct {
	DisplayName      string
	RoleID           string
	TeamIDs          []string
	HandlesCustomers bool
	AvatarFileID     string
	Execution        ExecutionInput
}

// UpdateInput 定义 AI 员工可编辑字段，AvatarFileID 为空时保留当前头像。
type UpdateInput struct {
	DisplayName      string
	RoleID           string
	TeamIDs          []string
	HandlesCustomers bool
	WorkStatus       domain.WorkStatus
	AvatarFileID     string
}

// TeamSummary 定义 AI 员工所属团队摘要。
type TeamSummary = teamaction.Summary

// ListInput 定义 AI 员工目录查询条件。
type ListInput struct {
	Query    string
	Status   domain.UserStatus
	Page     int
	PageSize int
}

// Agent 定义 AI 员工信息。
type Agent struct {
	ID               string            `bun:"id"`
	IdentityID       string            `bun:"identity_id"`
	DisplayName      string            `bun:"display_name"`
	AvatarFileID     *string           `bun:"avatar_file_id"`
	RoleID           string            `bun:"role_id"`
	RoleKind         domain.RoleKind   `bun:"role_kind"`
	RoleName         string            `bun:"role_name"`
	HandlesCustomers bool              `bun:"handles_customers"`
	Status           domain.UserStatus `bun:"status"`
	WorkStatus       domain.WorkStatus `bun:"work_status"`
	Teams            []TeamSummary
	Execution        Execution
	CreatedAt        time.Time `bun:"created_at"`
}

// ListItem 定义 AI 员工目录项。
type ListItem struct {
	ID               string            `bun:"id"`
	IdentityID       string            `bun:"identity_id"`
	DisplayName      string            `bun:"display_name"`
	AvatarFileID     *string           `bun:"avatar_file_id"`
	RoleID           string            `bun:"role_id"`
	RoleKind         domain.RoleKind   `bun:"role_kind"`
	RoleName         string            `bun:"role_name"`
	HandlesCustomers bool              `bun:"handles_customers"`
	Status           domain.UserStatus `bun:"status"`
	WorkStatus       domain.WorkStatus `bun:"work_status"`
	Teams            []TeamSummary
	Execution        ExecutionSummary
	CreatedAt        time.Time `bun:"created_at"`
}

// ListOutput 定义 AI 员工分页结果。
type ListOutput struct {
	Agents []ListItem
	Page   common.PageInfo
}
