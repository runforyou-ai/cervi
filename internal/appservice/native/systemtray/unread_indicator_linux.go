//go:build !server && linux

package systemtray

import (
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// newUnreadIndicator 为 Linux 保留未读消息指示器的统一能力占位。
func newUnreadIndicator(_ *application.App, _ *application.SystemTray) appservice.UnreadIndicator {
	return &noopUnreadIndicator{}
}
