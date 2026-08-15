//go:build server

package auth

import (
	"context"

	commonsession "github.com/runforyou-ai/cervi/internal/common/session"
)

// LogoutAction 执行用户退出登录操作。
type LogoutAction struct {
	sessions *commonsession.Manager
}

// NewLogoutAction 创建用户退出登录操作。
func NewLogoutAction(sessions *commonsession.Manager) *LogoutAction {
	return &LogoutAction{sessions: sessions}
}

// Execute 删除当前令牌对应的登录会话。
func (a *LogoutAction) Execute(ctx context.Context, token string) error {
	return a.sessions.Revoke(ctx, token)
}
