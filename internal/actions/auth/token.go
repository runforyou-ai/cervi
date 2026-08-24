//go:build server

package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common/token"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// issueToken 签发登录令牌并返回对应身份。
func issueToken(ctx context.Context, db bun.IDB, userID string) (token.Issued, *servermodels.Identity, error) {
	issued, err := token.Issue()
	if err != nil {
		return token.Issued{}, nil, fmt.Errorf("issue token: %w", err)
	}
	record := &servermodels.Token{
		UserID:    userID,
		TokenHash: issued.TokenHash,
		ExpiresAt: issued.ExpiresAt,
	}
	if _, err := db.NewInsert().
		Model(record).
		Column("user_id", "token_hash", "expires_at").
		Exec(ctx); err != nil {
		return token.Issued{}, nil, fmt.Errorf("save token: %w", err)
	}

	identity, err := resolveIdentity(ctx, db, issued.Token)
	if err != nil {
		return token.Issued{}, nil, fmt.Errorf("find token identity: %w", err)
	}
	if identity == nil {
		return token.Issued{}, nil, errors.New("find token identity: empty result")
	}
	return issued, identity, nil
}

// resolveIdentity 返回有效令牌对应的用户身份。
func resolveIdentity(ctx context.Context, db bun.IDB, value string) (*servermodels.Identity, error) {
	identity := &servermodels.Identity{}
	err := db.NewRaw(`
		SELECT
			o.id::text,
			o.name,
			u.id::text,
			u.identity_id::text,
			u.organization_id::text,
			u.email,
			u.role_id::text,
			u.status,
			u.locale,
			u.time_zone,
			u.message_notifications_enabled,
			oi.id::text,
			oi.organization_id::text,
			oi.type,
			oi.display_name,
			oi.avatar_file_id::text,
			oi.work_status
		FROM tokens AS token
		JOIN users AS u ON u.id = token.user_id
		JOIN organization_identities AS oi ON oi.id = u.identity_id AND oi.organization_id = u.organization_id AND oi.type = 'user'
		JOIN organizations AS o ON o.id = u.organization_id
		WHERE token.token_hash = ?
		  AND token.expires_at > now()
		LIMIT 1
	`, token.Hash(value)).Scan(
		ctx,
		&identity.Organization.ID,
		&identity.Organization.Name,
		&identity.User.ID,
		&identity.User.IdentityID,
		&identity.User.OrganizationID,
		&identity.User.Email,
		&identity.User.RoleID,
		&identity.User.Status,
		&identity.User.Locale,
		&identity.User.TimeZone,
		&identity.User.MessageNotificationsEnabled,
		&identity.OrganizationIdentity.ID,
		&identity.OrganizationIdentity.OrganizationID,
		&identity.OrganizationIdentity.Type,
		&identity.OrganizationIdentity.DisplayName,
		&identity.OrganizationIdentity.AvatarFileID,
		&identity.OrganizationIdentity.WorkStatus,
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

// revokeToken 删除令牌记录。
func revokeToken(ctx context.Context, db *bun.DB, value string) error {
	_, err := db.NewDelete().
		Model((*servermodels.Token)(nil)).
		Where("token_hash = ?", token.Hash(value)).
		Exec(ctx)
	return err
}
