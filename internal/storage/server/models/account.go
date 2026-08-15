//go:build server

// Package models 定义 PostgreSQL 使用的 Bun 数据模型。
package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Organization 表示 PostgreSQL 中的企业组织。
type Organization struct {
	bun.BaseModel `bun:"table:organizations,alias:o"`

	ID   string `bun:"id,pk" json:"id"`
	Name string `bun:"name" json:"name"`
}

// User 表示 PostgreSQL 中的企业成员。
type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID             string `bun:"id,pk" json:"id"`
	OrganizationID string `bun:"organization_id" json:"organizationId"`
	Email          string `bun:"email" json:"email"`
	DisplayName    string `bun:"display_name" json:"displayName"`
	PasswordHash   string `bun:"password_hash" json:"-"`
	Role           string `bun:"role" json:"role"`
	Status         string `bun:"status" json:"status"`
}

// Session 表示 PostgreSQL 中的用户登录会话。
type Session struct {
	bun.BaseModel `bun:"table:sessions,alias:s"`

	UserID    string    `bun:"user_id"`
	TokenHash string    `bun:"token_hash"`
	ExpiresAt time.Time `bun:"expires_at"`
}

// Principal 表示当前用户及其所属企业。
type Principal struct {
	Organization Organization `json:"organization"`
	User         User         `json:"user"`
}
