//go:build !server && (android || ios)

package main

import (
	"github.com/runforyou-ai/cervi/internal/appservice"
	appservicenative "github.com/runforyou-ai/cervi/internal/appservice/native"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// setupDesktopSystemTray 保持移动端不启用桌面托盘能力。
func setupDesktopSystemTray(_ *application.App, _ application.Window, _ func()) appservice.UnreadIndicator {
	return appservicenative.NewNoopUnreadIndicator()
}
