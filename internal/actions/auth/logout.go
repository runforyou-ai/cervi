//go:build server

package auth

import (
	"context"

	"github.com/uptrace/bun"
)

// LogoutAction 执行用户退出登录操作。
type LogoutAction struct {
	db *bun.DB
}

// NewLogoutAction 创建用户退出登录操作。
func NewLogoutAction(db *bun.DB) *LogoutAction {
	return &LogoutAction{db: db}
}

// Execute 删除当前令牌对应的登录会话。
func (a *LogoutAction) Execute(ctx context.Context, token string) error {
	return revokeSession(ctx, a.db, token)
}
