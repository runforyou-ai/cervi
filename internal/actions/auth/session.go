//go:build server

package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common/sessiontoken"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// createSession 创建用户会话并返回对应身份。
func createSession(ctx context.Context, db *bun.DB, userID string) (sessiontoken.Issued, *servermodels.Identity, error) {
	issued, err := sessiontoken.Issue()
	if err != nil {
		return sessiontoken.Issued{}, nil, fmt.Errorf("issue session: %w", err)
	}
	record := &servermodels.Session{
		UserID:    userID,
		TokenHash: issued.TokenHash,
		ExpiresAt: issued.ExpiresAt,
	}
	if _, err := db.NewInsert().
		Model(record).
		Column("user_id", "token_hash", "expires_at").
		Exec(ctx); err != nil {
		return sessiontoken.Issued{}, nil, fmt.Errorf("save session: %w", err)
	}

	identity, err := resolveSession(ctx, db, issued.Token)
	if err != nil {
		return sessiontoken.Issued{}, nil, fmt.Errorf("find session identity: %w", err)
	}
	if identity == nil {
		return sessiontoken.Issued{}, nil, errors.New("find session identity: empty result")
	}
	return issued, identity, nil
}

// resolveSession 返回有效令牌对应的用户身份。
func resolveSession(ctx context.Context, db *bun.DB, token string) (*servermodels.Identity, error) {
	identity := &servermodels.Identity{}
	err := db.NewRaw(`
		SELECT
			o.id::text,
			o.name,
			u.id::text,
			u.organization_id::text,
			u.email,
			u.display_name,
			u.role,
			u.status
		FROM sessions AS s
		JOIN users AS u ON u.id = s.user_id
		JOIN organizations AS o ON o.id = u.organization_id
		WHERE s.token_hash = ?
		  AND s.expires_at > now()
		LIMIT 1
	`, sessiontoken.Hash(token)).Scan(
		ctx,
		&identity.Organization.ID,
		&identity.Organization.Name,
		&identity.User.ID,
		&identity.User.OrganizationID,
		&identity.User.Email,
		&identity.User.DisplayName,
		&identity.User.Role,
		&identity.User.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if identity.User.Status != "active" {
		return nil, nil
	}
	return identity, nil
}

// revokeSession 删除令牌对应的登录会话。
func revokeSession(ctx context.Context, db *bun.DB, token string) error {
	_, err := db.NewDelete().
		Model((*servermodels.Session)(nil)).
		Where("token_hash = ?", sessiontoken.Hash(token)).
		Exec(ctx)
	return err
}
