//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Session 表示 PostgreSQL 中的用户登录会话。
type Session struct {
	bun.BaseModel `bun:"table:sessions,alias:s"`

	UserID    string    `bun:"user_id"`
	TokenHash string    `bun:"token_hash"`
	ExpiresAt time.Time `bun:"expires_at"`
}
