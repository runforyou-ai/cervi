//go:build !server && !android && !ios

package main

import (
	_ "embed"
	"runtime"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed build/appicon.png
var desktopTrayIcon []byte

//go:embed build/appicon.icon/Assets/cervi_icon.png
var macDesktopTrayTemplateIcon []byte

// setupDesktopSystemTray 配置桌面端托盘、托盘菜单和关闭窗口时隐藏到托盘的行为。
func setupDesktopSystemTray(app *application.App, window application.Window, requestQuit func()) appservice.UnreadIndicator {
	showWindow := func() {
		if window.IsMinimised() {
			window.UnMinimise()
		}
		window.Show().Focus()
	}

	trayMenu := app.Menu.New()
	trayMenu.Add("打开 Cervi").OnClick(func(_ *application.Context) {
		showWindow()
	})
	trayMenu.Add("退出 Cervi").OnClick(func(_ *application.Context) {
		requestQuit()
		app.Quit()
	})

	tray := app.SystemTray.New()
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(macDesktopTrayTemplateIcon)
	} else {
		tray.SetIcon(desktopTrayIcon)
	}
	tray.SetTooltip("Cervi")
	tray.SetMenu(trayMenu)
	tray.OnClick(showWindow)
	tray.OnRightClick(tray.OpenMenu)

	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		window.Hide()
	})

	return newPlatformUnreadIndicator(app, tray, desktopTrayIcon)
}
