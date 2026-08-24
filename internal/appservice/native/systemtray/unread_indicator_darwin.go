//go:build !server && darwin && !ios

package systemtray

import (
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
)

// darwinUnreadIndicator 同步更新 macOS 菜单栏托盘文字和 Dock 角标。
type darwinUnreadIndicator struct {
	tray *application.SystemTray
	dock *dock.DockService
}

// newUnreadIndicator 创建 macOS 未读消息指示器并注册 Dock 服务。
func newUnreadIndicator(app *application.App, tray *application.SystemTray) appservice.UnreadIndicator {
	dockService := dock.New()
	app.RegisterService(application.NewService(dockService))
	return &darwinUnreadIndicator{tray: tray, dock: dockService}
}

// SetUnreadCount 设置菜单栏托盘文字和 Dock 角标中的绝对未读消息数。
func (i *darwinUnreadIndicator) SetUnreadCount(count int) error {
	label := appservice.FormatUnreadCount(count)
	i.tray.SetLabel(label)
	if label == "" {
		return i.dock.RemoveBadge()
	}
	return i.dock.SetBadge(label)
}
