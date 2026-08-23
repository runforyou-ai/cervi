//go:build server

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// run 解析服务端运行参数并启动 HTTP 服务。
func run(arguments []string) error {
	flags := flag.NewFlagSet("cervi-server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", "", "显式指定 YAML 配置文件")
	checkConfig := flags.Bool("check-config", false, "校验配置后退出")
	showVersion := flags.Bool("version", false, "显示版本后退出")
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("解析服务端参数: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("不支持位置参数 %q", flags.Arg(0))
	}
	if *showVersion {
		_, err := fmt.Fprintln(os.Stdout, version)
		return err
	}

	config, err := serverconfig.Load(*configPath)
	if err != nil {
		return fmt.Errorf("加载服务端配置: %w", err)
	}
	if *checkConfig {
		_, err := fmt.Fprintln(os.Stdout, "服务端配置有效")
		return err
	}

	appStorage, err := storage.Open(context.Background(), config.Database)
	if err != nil {
		return fmt.Errorf("initialize storage: %w", err)
	}
	defer func() {
		if err := appStorage.Close(); err != nil {
			slog.Warn("关闭存储失败", "error", err)
		}
	}()

	services, err := applicationServices(appStorage, config)
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
			Host: config.Server.Host,
			Port: config.Server.Port,
		},
	})

	slog.Info("启动 Cervi 服务端", "version", version, "host", config.Server.Host, "port", config.Server.Port, "environment", config.Environment)
	return app.Run()
}
