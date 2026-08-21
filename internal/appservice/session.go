package appservice

import (
	"context"
	"log/slog"
	"strings"
)

// LoadSession 返回当前应进入的会话入口。
func (s *Service) LoadSession(ctx context.Context, meta RequestMeta) (Session, error) {
	var session Session
	var err error
	if _, ok := s.backend.(ServerConnector); ok {
		session, err = s.loadNativeSession(ctx, meta)
	} else {
		session, err = s.loadWebSession(ctx, meta)
	}
	if err != nil {
		return Session{}, err
	}
	slog.Info("解析会话入口", "state", session.State)
	return session, nil
}

// loadWebSession 按企业初始化与登录状态选择 Web 入口。
func (s *Service) loadWebSession(ctx context.Context, meta RequestMeta) (Session, error) {
	status, err := s.backend.InstallationStatus(ctx, meta)
	if err != nil {
		return Session{}, err
	}
	name := strings.TrimSpace(status.OrganizationName)
	if !status.Installed || name == "" {
		return Session{State: SessionStateSetup}, nil
	}
	if meta.Token == "" {
		return Session{State: SessionStateLogin, OrganizationName: name}, nil
	}
	identity, err := s.backend.LoadIdentity(ctx, meta)
	if err != nil {
		return sessionFromError(err, name, false)
	}
	return Session{State: SessionStateReady, Identity: &identity}, nil
}

// loadNativeSession 按服务器连接与登录状态选择原生端入口。
func (s *Service) loadNativeSession(ctx context.Context, meta RequestMeta) (Session, error) {
	connector := s.backend.(ServerConnector)
	serverURL, err := connector.ServerURL(ctx, meta)
	if err != nil {
		return Session{}, err
	}
	if serverURL == "" {
		return Session{State: SessionStateConnect}, nil
	}
	status, err := s.backend.InstallationStatus(ctx, meta)
	if err != nil {
		session, convErr := sessionFromError(err, "", true)
		if convErr != nil {
			return Session{}, convErr
		}
		return session, nil
	}
	name := strings.TrimSpace(status.OrganizationName)
	if !status.Installed || name == "" {
		return Session{State: SessionStateConnect}, nil
	}
	if meta.Token == "" {
		return Session{State: SessionStateLogin, OrganizationName: name}, nil
	}
	identity, err := s.backend.LoadIdentity(ctx, meta)
	if err != nil {
		session, convErr := sessionFromError(err, name, true)
		if convErr != nil {
			return Session{}, convErr
		}
		if session.State == SessionStateConnect {
			slog.Warn("无法读取登录身份，进入连接页", "error", err)
		}
		return session, nil
	}
	return Session{State: SessionStateReady, Identity: &identity}, nil
}

// sessionFromError 把会话类错误转换成入口；无法转换则原样返回错误。
func sessionFromError(err error, organizationName string, native bool) (Session, error) {
	state := SessionStateOf(err)
	if native && (state == SessionStateSetup || state == SessionStateConnect) {
		return Session{State: SessionStateConnect}, nil
	}
	if state == SessionStateLogin {
		return Session{State: SessionStateLogin, OrganizationName: organizationName}, nil
	}
	if state != "" {
		return Session{State: state}, nil
	}
	return Session{}, err
}
