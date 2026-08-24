//go:build !server && linux

package main

import (
	"github.com/runforyou-ai/cervi/internal/appservice"
	appservicenative "github.com/runforyou-ai/cervi/internal/appservice/native"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// newPlatformUnreadIndicator 为 Linux 保留未读消息指示器的统一能力占位。
func newPlatformUnreadIndicator(_ *application.App, _ *application.SystemTray, _ []byte) appservice.UnreadIndicator {
	return appservicenative.NewNoopUnreadIndicator()
}
