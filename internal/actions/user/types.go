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
	Page     int
	PageSize int
}

// PageInfo 定义企业成员分页信息。
type PageInfo struct {
	Number int `json:"number"`
	Size   int `json:"size"`
	Total  int `json:"total"`
}

// DirectoryUser 定义团队成员目录字段。
type DirectoryUser struct {
	ID          string            `json:"id"`
	Email       string            `json:"email"`
	DisplayName string            `json:"displayName"`
	Role        domain.UserRole   `json:"role"`
	Status      domain.UserStatus `json:"status"`
	WorkStatus  domain.WorkStatus `json:"workStatus"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// ListOutput 定义企业成员分页结果。
type ListOutput struct {
	Users []DirectoryUser `json:"users"`
	Page  PageInfo        `json:"page"`
}
