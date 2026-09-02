//go:build server

package auth

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/common"
	commontoken "github.com/runforyou-ai/cervi/internal/common/token"
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

// Execute 删除当前登录令牌。
func (a *LogoutAction) Execute(ctx context.Context, organizationID string, token string) error {
	// 删除令牌记录。
	if !common.ValidUUID(organizationID) {
		return ErrIdentityNotFound
	}
	_, err := a.db.ExecContext(ctx, `
		DELETE FROM tokens AS token
		USING users AS u
		WHERE token.user_id = u.id
		  AND u.organization_id = ?
		  AND token.token_hash = ?
	`, organizationID, commontoken.Hash(token))
	return err
}
