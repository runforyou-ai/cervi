// Package clientsession 管理原生端当前登录凭据。
package clientsession

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	if err := validateCredential(credential); err != nil {
		return nil, fmt.Errorf("validate saved client session: %w", err)
	}
	if !credential.ExpiresAt.After(time.Now()) {
		if err := store.DeleteClientSession(ctx); err != nil {
			return nil, fmt.Errorf("delete expired client session: %w", err)
		}
		return manager, nil
	}
	manager.current = &credential
	return manager, nil
}

// Current 返回指定企业服务器当前有效的登录凭据。
func (m *Manager) Current(ctx context.Context, serverURL string) (Credential, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil || m.current.ServerURL != strings.TrimSpace(serverURL) {
		return Credential{}, false, nil
	}
	if m.current.ExpiresAt.After(time.Now()) {
		return *m.current, true, nil
	}
	m.current = nil
	if err := m.store.DeleteClientSession(ctx); err != nil {
		return Credential{}, false, fmt.Errorf("delete expired client session: %w", err)
	}
	return Credential{}, false, nil
}

// Establish 保存并启用新的原生端登录凭据。
func (m *Manager) Establish(ctx context.Context, credential Credential) error {
	credential.ServerURL = strings.TrimSpace(credential.ServerURL)
	if err := validateCredential(credential); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.SaveClientSession(ctx, credential); err != nil {
		return fmt.Errorf("save client session: %w", err)
	}
	m.current = &credential
	return nil
}

// Clear 删除原生端当前登录凭据。
func (m *Manager) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.store.DeleteClientSession(ctx)
	m.current = nil
	if err != nil {
		return fmt.Errorf("delete client session: %w", err)
	}
	return nil
}

// ClearIfCurrent 仅在被拒绝的凭据仍是当前会话时删除它。
func (m *Manager) ClearIfCurrent(ctx context.Context, rejected Credential) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil ||
		m.current.ServerURL != rejected.ServerURL ||
		m.current.OrganizationID != rejected.OrganizationID ||
		m.current.UserID != rejected.UserID ||
		m.current.Token != rejected.Token {
		return false, nil
	}
	err := m.store.DeleteClientSession(ctx)
	m.current = nil
	if err != nil {
		return false, fmt.Errorf("delete rejected client session: %w", err)
	}
	return true, nil
}

// validateCredential 校验登录凭据包含完整的持久化字段。
func validateCredential(credential Credential) error {
	switch {
	case strings.TrimSpace(credential.ServerURL) == "":
		return errors.New("client session server URL is empty")
	case strings.TrimSpace(credential.OrganizationID) == "":
		return errors.New("client session organization ID is empty")
	case strings.TrimSpace(credential.UserID) == "":
		return errors.New("client session user ID is empty")
	case strings.TrimSpace(credential.Token) == "":
		return errors.New("client session token is empty")
	case credential.ExpiresAt.IsZero():
		return errors.New("client session expiration is empty")
	default:
		return nil
	}
}
