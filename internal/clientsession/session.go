// Package clientsession 管理原生端当前登录凭据。
package clientsession

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Credential 表示原生端当前登录凭据及所属范围。
type Credential struct {
	ServerURL      string
	OrganizationID string
	UserID         string
	Token          string
	ExpiresAt      time.Time
}

// Store 持久化原生端当前登录凭据。
type Store interface {
	// LoadClientSession 读取当前登录凭据。
	LoadClientSession(context.Context) (Credential, bool, error)
	// SaveClientSession 保存当前登录凭据。
	SaveClientSession(context.Context, Credential) error
	// DeleteClientSession 删除当前登录凭据。
	DeleteClientSession(context.Context) error
}

// Manager 维护原生端当前登录凭据及其进程内缓存。
type Manager struct {
	store   Store
	mu      sync.Mutex
	current *Credential
}

// NewManager 从持久化存储恢复原生端当前登录凭据。
func NewManager(ctx context.Context, store Store) (*Manager, error) {
	credential, found, err := store.LoadClientSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("load client session: %w", err)
	}
	manager := &Manager{store: store}
	if !found {
		return manager, nil
	}
	if !credential.ExpiresAt.After(time.Now()) {
		if err := store.DeleteClientSession(ctx); err != nil {
			slog.Warn("删除过期的原生端登录会话失败", "server_url", credential.ServerURL, "organization_id", credential.OrganizationID, "user_id", credential.UserID, "error", err)
			return manager, nil
		}
		slog.Info("已删除过期的原生端登录会话", "server_url", credential.ServerURL, "organization_id", credential.OrganizationID, "user_id", credential.UserID)
		return manager, nil
	}
	manager.current = &credential
	slog.Info("已恢复原生端登录会话", "server_url", credential.ServerURL, "organization_id", credential.OrganizationID, "user_id", credential.UserID, "expires_at", credential.ExpiresAt)
	return manager, nil
}

// Current 返回指定企业服务器当前有效的登录凭据。
func (m *Manager) Current(ctx context.Context, serverURL string) (Credential, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil || m.current.ServerURL != serverURL {
		return Credential{}, false
	}
	if m.current.ExpiresAt.After(time.Now()) {
		return *m.current, true
	}
	expired := *m.current
	m.current = nil
	if err := m.store.DeleteClientSession(ctx); err != nil {
		slog.Warn("删除过期的原生端登录会话失败", "server_url", expired.ServerURL, "organization_id", expired.OrganizationID, "user_id", expired.UserID, "error", err)
		return Credential{}, false
	}
	slog.Info("已删除过期的原生端登录会话", "server_url", expired.ServerURL, "organization_id", expired.OrganizationID, "user_id", expired.UserID)
	return Credential{}, false
}

// Establish 保存并启用新的原生端登录凭据。
func (m *Manager) Establish(ctx context.Context, credential Credential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.SaveClientSession(ctx, credential); err != nil {
		return fmt.Errorf("save client session: %w", err)
	}
	m.current = &credential
	slog.Info("原生端登录会话已建立", "server_url", credential.ServerURL, "organization_id", credential.OrganizationID, "user_id", credential.UserID, "expires_at", credential.ExpiresAt)
	return nil
}

// Clear 删除原生端当前登录凭据。
func (m *Manager) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	credential := m.current
	if err := m.store.DeleteClientSession(ctx); err != nil {
		return fmt.Errorf("delete client session: %w", err)
	}
	m.current = nil
	if credential != nil {
		slog.Info("原生端登录会话已清除", "server_url", credential.ServerURL, "organization_id", credential.OrganizationID, "user_id", credential.UserID)
	}
	return nil
}

// ClearIfCurrent 仅在被拒绝的凭据仍是当前会话时删除它。
func (m *Manager) ClearIfCurrent(ctx context.Context, rejected Credential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil ||
		m.current.ServerURL != rejected.ServerURL ||
		m.current.Token != rejected.Token {
		return nil
	}
	if err := m.store.DeleteClientSession(ctx); err != nil {
		return fmt.Errorf("delete rejected client session: %w", err)
	}
	m.current = nil
	slog.Info("服务端拒绝原生端登录凭据，已清除会话", "server_url", rejected.ServerURL, "organization_id", rejected.OrganizationID, "user_id", rejected.UserID)
	return nil
}
