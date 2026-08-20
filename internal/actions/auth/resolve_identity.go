//go:build server

package auth

import (
	"context"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ResolveIdentityQuery 解析当前登录令牌对应的身份。
type ResolveIdentityQuery struct {
	db *bun.DB
}

// NewResolveIdentityQuery 创建登录身份查询。
func NewResolveIdentityQuery(db *bun.DB) *ResolveIdentityQuery {
	return &ResolveIdentityQuery{db: db}
}

// Execute 返回有效令牌对应的用户身份。
func (q *ResolveIdentityQuery) Execute(ctx context.Context, value string) (*servermodels.Identity, error) {
	return resolveIdentity(ctx, q.db, value)
}
