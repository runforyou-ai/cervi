//go:build server

package auth

import (
	"context"

	commonsession "github.com/runforyou-ai/cervi/internal/common/session"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ResolveSessionQuery 解析当前登录会话。
type ResolveSessionQuery struct {
	sessions *commonsession.Manager
}

// NewResolveSessionQuery 创建登录会话查询。
func NewResolveSessionQuery(sessions *commonsession.Manager) *ResolveSessionQuery {
	return &ResolveSessionQuery{sessions: sessions}
}

// Execute 返回有效令牌对应的用户身份。
func (q *ResolveSessionQuery) Execute(ctx context.Context, token string) (*servermodels.Principal, error) {
	return q.sessions.Resolve(ctx, token)
}
