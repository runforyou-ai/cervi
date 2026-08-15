//go:build !server

package main

import (
	"fmt"

	"github.com/runforyou-ai/cervi/internal/apiproxy"
	"github.com/runforyou-ai/cervi/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// newAPIProxyService 创建桌面端和移动端共用的 API 代理服务。
func newAPIProxyService(appStorage storage.Storage) (application.Service, error) {
	proxyStorage, ok := appStorage.(apiproxy.Store)
	if !ok {
		return application.Service{}, fmt.Errorf("storage does not provide enterprise server configuration")
	}

	apiService, err := apiproxy.NewService(proxyStorage)
	if err != nil {
		return application.Service{}, fmt.Errorf("initialize API proxy: %w", err)
	}
	return application.NewServiceWithOptions(apiService, application.ServiceOptions{
		Route: "/api",
	}), nil
}
