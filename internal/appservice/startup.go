package appservice

import (
	"context"
	"log/slog"
	"strings"
)

// LoadStartup 返回初始化或服务器连接入口，不读取登录状态。
func (s *Service) LoadStartup(ctx context.Context, meta RequestMeta) (Startup, error) {
	var startup Startup
	var err error
	if connector, ok := s.backend.(ServerConnector); ok {
		startup, err = s.loadNativeStartup(ctx, meta, connector)
	} else {
		// 根据企业初始化状态选择 Web 启动入口。
		status, statusErr := s.backend.InstallationStatus(ctx, meta)
		if statusErr != nil {
			return Startup{}, statusErr
		}
		name := strings.TrimSpace(status.OrganizationName)
		if !status.Installed || name == "" {
			startup = Startup{State: SessionStateSetup}
		} else {
			startup = Startup{State: SessionStateReady, OrganizationName: name}
		}
	}
	if err != nil {
		return Startup{}, err
	}
	slog.Info("应用启动检测完成", "state", startup.State)
	return startup, nil
}

// loadNativeStartup 检测原生端已保存企业服务器的连通和初始化状态。
func (s *Service) loadNativeStartup(ctx context.Context, meta RequestMeta, connector ServerConnector) (Startup, error) {
	serverURL, err := connector.ServerURL(ctx, meta)
	if err != nil {
		slog.Warn("读取企业服务器地址失败，进入连接页", "error", err)
		return Startup{State: SessionStateConnect}, nil
	}
	if serverURL == "" {
		return Startup{State: SessionStateConnect}, nil
	}
	status, err := s.backend.InstallationStatus(ctx, meta)
	if err != nil {
		return Startup{State: SessionStateConnect}, nil
	}
	name := strings.TrimSpace(status.OrganizationName)
	if !status.Installed || name == "" {
		return Startup{State: SessionStateConnect}, nil
	}
	return Startup{State: SessionStateReady, OrganizationName: name}, nil
}
