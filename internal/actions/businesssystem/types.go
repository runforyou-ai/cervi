//go:build server

// Package businesssystem 实现企业业务系统的查询与管理。
package businesssystem

import "time"

// Input 定义业务系统可编辑字段。
type Input struct {
	Name    string
	URL     string
	Enabled bool
}

// Record 定义业务系统记录。
type Record struct {
	ID        string
	Name      string
	URL       string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
