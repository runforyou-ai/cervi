//go:build !server

package main

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync/atomic"

	"github.com/runforyou-ai/cervi/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// run 初始化原生端存储与应用服务，并运行 Wails 应用。
func run(_ []string) error {
	appStorage, err := storage.Open(context.Background())
	if err != nil {
		return fmt.Errorf("initialize storage: %w", err)
	}
	defer func() {
		if err := appStorage.Close(); err != nil {
			slog.Warn("关闭存储失败", "error", err)
		}
	}()

	services, err := applicationServices(appStorage)
	if err != nil {
		return fmt.Errorf("initialize application services: %w", err)
	}

	var trayQuitRequested atomic.Bool
	app := application.New(application.Options{
		Name:        "Cervi",
		Description: "Cervi is an open-source AI customer support teammate platform",
		Services:    services,
		ShouldQuit: func() bool {
			if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
				return true
			}
			return trayQuitRequested.Load()
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Server: application.ServerOptions{
			Port: 8080,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{
			DisableQuitOnLastWindowClosed: true,
		},
	})

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Cervi",
		Width:            1440,
		Height:           900,
		MinWidth:         1440,
		MinHeight:        900,
		BackgroundColour: application.NewRGB(250, 250, 250),
		URL:              "/",
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarHidden,
		},
	})
	unreadIndicator := setupDesktopSystemTray(app, mainWindow, func() {
		trayQuitRequested.Store(true)
	})
	_ = unreadIndicator
	// 临时未读角标演示需要平台验证时再启用。
	// startUnreadIndicatorDemo(app, unreadIndicator)

	slog.Info("启动 Cervi")
	return app.Run()
}
