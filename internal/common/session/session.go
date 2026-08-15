//go:build server

// Package session 提供登录会话的创建、解析和撤销能力。
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const defaultDuration = 30 * 24 * time.Hour

// Issued 表示新生成的会话凭据。
type Issued struct {
	Token     string
	TokenHash string
	ExpiresAt time.Time
}

// Manager 管理持久化登录会话。
type Manager struct {
	db *bun.DB
}

// NewManager 创建登录会话管理器。
func NewManager(db *bun.DB) *Manager {
	return &Manager{db: db}
}

// Issue 生成新的会话凭据。
func Issue() (Issued, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return Issued{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(buffer)
	return Issued{
		Token:     token,
		TokenHash: HashToken(token),
		ExpiresAt: time.Now().Add(defaultDuration),
	}, nil
}

// Create 生成并保存用户登录会话。
func (m *Manager) Create(ctx context.Context, userID string) (Issued, *servermodels.Principal, error) {
	issued, err := Issue()
	if err != nil {
		return Issued{}, nil, fmt.Errorf("issue session: %w", err)
	}
	record := &servermodels.Session{
		UserID:    userID,
		TokenHash: issued.TokenHash,
		ExpiresAt: issued.ExpiresAt,
	}
	if _, err := m.db.NewInsert().
		Model(record).
		Column("user_id", "token_hash", "expires_at").
		Exec(ctx); err != nil {
		return Issued{}, nil, fmt.Errorf("save session: %w", err)
	}

	principal, err := m.Resolve(ctx, issued.Token)
	if err != nil {
		return Issued{}, nil, fmt.Errorf("find session principal: %w", err)
	}
	if principal == nil {
		return Issued{}, nil, errors.New("find session principal: empty result")
	}
	return issued, principal, nil
}

// Resolve 返回有效令牌对应的用户身份。
func (m *Manager) Resolve(ctx context.Context, token string) (*servermodels.Principal, error) {
	principal := &servermodels.Principal{}
	err := m.db.NewRaw(`
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
	`, HashToken(token)).Scan(
		ctx,
		&principal.Organization.ID,
		&principal.Organization.Name,
		&principal.User.ID,
		&principal.User.OrganizationID,
		&principal.User.Email,
		&principal.User.DisplayName,
		&principal.User.Role,
		&principal.User.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if principal.User.Status != "active" {
		return nil, nil
	}
	return principal, nil
}

// Revoke 删除当前令牌对应的登录会话。
func (m *Manager) Revoke(ctx context.Context, token string) error {
	_, err := m.db.NewDelete().
		Model((*servermodels.Session)(nil)).
		Where("token_hash = ?", HashToken(token)).
		Exec(ctx)
	return err
}

// HashToken 计算会话令牌的 SHA-256 哈希。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
