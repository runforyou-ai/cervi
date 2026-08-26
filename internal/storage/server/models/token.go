//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Token 表示 PostgreSQL 中的登录令牌。
type Token struct {
	bun.BaseModel `bun:"table:tokens,alias:token"`

	UserID    string    `bun:"user_id"`
	TokenHash string    `bun:"token_hash"`
	ExpiresAt time.Time `bun:"expires_at"`
	CreatedAt time.Time `bun:"created_at"`
}
