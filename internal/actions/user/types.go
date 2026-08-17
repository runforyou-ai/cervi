//go:build server

package user

import "time"

// ListInput 定义企业成员目录查询条件。
type ListInput struct {
	Query    string
	Status   string
	Role     string
	Page     int
	PageSize int
}

// PageInfo 定义企业成员分页信息。
type PageInfo struct {
	Number int `json:"number"`
	Size   int `json:"size"`
	Total  int `json:"total"`
}

// DirectoryUser 定义联系人目录展示所需的内部用户字段。
type DirectoryUser struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Email          string    `json:"email"`
	DisplayName    string    `json:"displayName"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
}

// ListOutput 定义企业成员分页结果。
type ListOutput struct {
	Users []DirectoryUser `json:"users"`
	Page  PageInfo        `json:"page"`
}
