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

func main() {
	if err := run(); err != nil {
		slog.Error("Cervi 运行失败", "error", err)
		os.Exit(1)
	}
}

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

	app := application.New(application.Options{
		Name:        "Cervi",
		Description: "Cervi is an open-source AI customer support teammate platform",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// Server 构建默认监听 8080 端口，地址可通过 WAILS_SERVER_HOST 覆盖。
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
		MinWidth:         1080,
		MinHeight:        680,
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	slog.Info("正在启动 Cervi")
	return app.Run()
}
