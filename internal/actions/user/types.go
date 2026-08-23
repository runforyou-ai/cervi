//go:build server

package user

import (
	"time"

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

// PageInfo 定义企业成员分页信息。
type PageInfo struct {
	Number int `json:"number"`
	Size   int `json:"size"`
	Total  int `json:"total"`
}

// DirectoryUser 定义企业成员目录字段。
type DirectoryUser struct {
	ID          string            `json:"id"`
	IdentityID  string            `json:"-" bun:"identity_id"`
	Email       string            `json:"email"`
	DisplayName string            `json:"displayName"`
	RoleID      string            `json:"roleId" bun:"role_id"`
	RoleKind    domain.RoleKind   `json:"roleKind" bun:"role_kind"`
	RoleName    string            `json:"roleName" bun:"role_name"`
	Status      domain.UserStatus `json:"status"`
	WorkStatus  domain.WorkStatus `json:"workStatus"`
	Teams       []TeamSummary     `json:"teams"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// TeamSummary 定义成员所属团队的精简字段。
type TeamSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListOutput 定义企业成员分页结果。
type ListOutput struct {
	Users []DirectoryUser `json:"users"`
	Page  PageInfo        `json:"page"`
}
