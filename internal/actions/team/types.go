//go:build server

package team

import (
	"time"
)

// Input 定义团队可编辑字段。
type Input struct {
	Name        string
	Description string
}

// ListInput 定义团队列表查询条件。
type ListInput struct {
	Query    string
	Page     int
	PageSize int
}

// TeamRecord 定义团队详情字段。
type TeamRecord struct {
	ID          string    `bun:"id"`
	Name        string    `bun:"name"`
	Description string    `bun:"description"`
	MemberCount int       `bun:"member_count"`
	CreatedAt   time.Time `bun:"created_at"`
	UpdatedAt   time.Time `bun:"updated_at"`
}

// PageInfo 定义分页信息。
type PageInfo struct {
	Number int
	Size   int
	Total  int
}

// ListOutput 定义团队分页结果。
type ListOutput struct {
	Teams []TeamRecord
	Page  PageInfo
}
