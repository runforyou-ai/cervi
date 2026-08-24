//go:build !server && !android && !ios

package systemtray

import (
	"runtime"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
)

// Setup 配置桌面端托盘、托盘菜单和关闭窗口时隐藏到托盘的行为。
func Setup(options Options) appservice.UnreadIndicator {
	showWindow := func() {
		if options.Window.IsMinimised() {
			options.Window.UnMinimise()
		}
		options.Window.Show().Focus()
	}

	trayMenu := options.App.Menu.New()
	trayMenu.Add("打开 Cervi").OnClick(func(_ *application.Context) {
		showWindow()
	})
	trayMenu.Add("退出 Cervi").OnClick(func(_ *application.Context) {
		options.RequestQuit()
		options.App.Quit()
	})

	tray := options.App.SystemTray.New()
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(options.MacTemplateIcon)
	} else {
		tray.SetIcon(options.Icon)
	}
	tray.SetTooltip("Cervi")
	tray.SetMenu(trayMenu)
	tray.OnClick(showWindow)
	tray.OnRightClick(tray.OpenMenu)

	options.Window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		options.Window.Hide()
	})

	dockService := dock.New()
	options.App.RegisterService(application.NewService(dockService))
	return newUnreadIndicator(options.App, tray, dockService)
}
