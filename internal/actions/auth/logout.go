//go:build server

package auth

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/common"
	commontoken "github.com/runforyou-ai/cervi/internal/common/token"
	"github.com/runforyou-ai/cervi/internal/realtime"
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

// Execute 删除当前登录令牌，提交后通知 Gateway 关闭该登录会话的实时连接。
func (a *LogoutAction) Execute(ctx context.Context, organizationID string, token string) error {
	if !common.ValidUUID(organizationID) {
		return ErrIdentityNotFound
	}
	return realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		var revoked []struct {
			ID     string `bun:"id"`
			UserID string `bun:"user_id"`
		}
		if err := tx.NewRaw(`
			DELETE FROM tokens AS token
			USING users AS u
			WHERE token.user_id = u.id
			  AND u.organization_id = ?
			  AND token.token_hash = ?
			RETURNING token.id::text AS id, token.user_id::text AS user_id
		`, organizationID, commontoken.Hash(token)).Scan(ctx, &revoked); err != nil {
			return err
		}
		for _, session := range revoked {
			realtime.Notify(ctx, realtime.UserSessionLoggedOut(organizationID, session.UserID, session.ID))
		}
		return nil
	})
}
