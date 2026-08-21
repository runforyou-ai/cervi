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
	Role     domain.UserRole
	TeamID   string
	Page     int
	PageSize int
}

// CreateInput 定义新增企业成员字段。
type CreateInput struct {
	DisplayName string
	Email       string
	Password    string
	Role        domain.UserRole
	TeamIDs     []string
}

// UpdateInput 定义企业成员可编辑字段。
type UpdateInput struct {
	DisplayName string
	Email       string
	Role        domain.UserRole
	TeamIDs     []string
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
	Email       string            `json:"email"`
	DisplayName string            `json:"displayName"`
	Role        domain.UserRole   `json:"role"`
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
