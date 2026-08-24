//go:build !server && !ios && !android

package desktop

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/runforyou-ai/cervi/internal/clientsession"
	desktopmodels "github.com/runforyou-ai/cervi/internal/storage/desktop/models"
)

const currentClientSessionID = "current"

// LoadClientSession 读取桌面端当前登录凭据。
func (s *Store) LoadClientSession(ctx context.Context) (clientsession.Credential, bool, error) {
	record := &desktopmodels.ClientSession{}
	err := s.db.NewSelect().
		Model(record).
		Where("id = ?", currentClientSessionID).
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return clientsession.Credential{}, false, nil
	}
	if err != nil {
		return clientsession.Credential{}, false, err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, record.ExpiresAt)
	if err != nil {
		return clientsession.Credential{}, false, fmt.Errorf("parse client session expiration: %w", err)
	}
	return clientSessionCredential(record.ServerURL, record.OrganizationID, record.UserID, record.Token, expiresAt), true, nil
}

// SaveClientSession 保存桌面端当前登录凭据。
func (s *Store) SaveClientSession(ctx context.Context, credential clientsession.Credential) error {
	record := &desktopmodels.ClientSession{
		ID:             currentClientSessionID,
		ServerURL:      credential.ServerURL,
		OrganizationID: credential.OrganizationID,
		UserID:         credential.UserID,
		Token:          credential.Token,
		ExpiresAt:      credential.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}
	_, err := s.db.NewInsert().
		Model(record).
		Column("id", "server_url", "organization_id", "user_id", "token", "expires_at").
		On("CONFLICT (id) DO UPDATE").
		Set("server_url = EXCLUDED.server_url").
		Set("organization_id = EXCLUDED.organization_id").
		Set("user_id = EXCLUDED.user_id").
		Set("token = EXCLUDED.token").
		Set("expires_at = EXCLUDED.expires_at").
		Set("updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	return err
}

// DeleteClientSession 删除桌面端当前登录凭据。
func (s *Store) DeleteClientSession(ctx context.Context) error {
	_, err := s.db.NewDelete().
		Model((*desktopmodels.ClientSession)(nil)).
		Where("id = ?", currentClientSessionID).
		Exec(ctx)
	return err
}

// clientSessionCredential 把桌面端存储记录转换成登录凭据。
func clientSessionCredential(serverURL, organizationID, userID, token string, expiresAt time.Time) clientsession.Credential {
	return clientsession.Credential{
		ServerURL:      serverURL,
		OrganizationID: organizationID,
		UserID:         userID,
		Token:          token,
		ExpiresAt:      expiresAt,
	}
}
