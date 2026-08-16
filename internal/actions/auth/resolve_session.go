//go:build server

package auth

import (
	"context"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ResolveSessionQuery 解析当前登录会话。
type ResolveSessionQuery struct {
	db *bun.DB
}

// NewResolveSessionQuery 创建登录会话查询。
func NewResolveSessionQuery(db *bun.DB) *ResolveSessionQuery {
	return &ResolveSessionQuery{db: db}
}

// Execute 返回有效令牌对应的用户身份。
func (q *ResolveSessionQuery) Execute(ctx context.Context, token string) (*servermodels.Principal, error) {
	return resolveSession(ctx, q.db, token)
}
