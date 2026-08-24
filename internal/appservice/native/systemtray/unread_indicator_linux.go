//go:build !server && linux && !android

package systemtray

import (
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
)

// newUnreadIndicator 为 Linux 保留未读消息指示器的统一能力占位。
func newUnreadIndicator(_ *application.App, _ *application.SystemTray, _ *dock.DockService) appservice.UnreadIndicator {
	return &noopUnreadIndicator{}
}
