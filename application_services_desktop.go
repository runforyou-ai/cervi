//go:build !server && !ios && !android

package main

import (
	"github.com/runforyou-ai/cervi/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// applicationServices 创建桌面端使用的 Wails 服务。
func applicationServices(appStorage storage.Storage) ([]application.Service, error) {
	proxyService, err := newAPIProxyService(appStorage)
	if err != nil {
		return nil, err
	}
	return []application.Service{proxyService}, nil
}
