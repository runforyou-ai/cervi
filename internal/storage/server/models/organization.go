//go:build server

// Package models 定义 PostgreSQL 使用的 Bun 数据模型。
package models

import "github.com/uptrace/bun"

// Organization 表示 PostgreSQL 中的企业组织。
type Organization struct {
	bun.BaseModel `bun:"table:organizations,alias:o"`

	ID   string `bun:"id,pk" json:"id"`
	Name string `bun:"name" json:"name"`
}
