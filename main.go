package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"

	"github.com/runforyou-ai/cervi/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// main 启动应用并记录无法恢复的运行错误。
func main() {
	if err := run(); err != nil {
		slog.Error("Cervi 运行失败", "error", err)
		os.Exit(1)
	}
}

// run 初始化存储与应用服务，并运行 Wails 应用。
func run() error {
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

	app := application.New(application.Options{
		Name:        "Cervi",
		Description: "Cervi is an open-source AI customer support teammate platform",
		Services:    services,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Server: application.ServerOptions{
			Port: 8080,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
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

	slog.Info("启动 Cervi")
	return app.Run()
}
