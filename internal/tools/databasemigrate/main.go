//go:build server

// Package main 为开发 Task 提供无需前端构建产物的数据库迁移入口。
package main

import (
	"context"
	"log/slog"
	"os"

	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
)

// main 加载工作区配置并通过服务端存储入口准备和迁移数据库。
func main() {
	config, err := serverconfig.Load("")
	if err != nil {
		slog.Error("加载服务端配置失败", "error", err)
		os.Exit(1)
	}
	store, err := serverstorage.Open(context.Background(), config.Database)
	if err != nil {
		slog.Error("准备和迁移数据库失败", "error", err)
		os.Exit(1)
	}
	if err := store.Close(); err != nil {
		slog.Error("关闭数据库失败", "error", err)
		os.Exit(1)
	}
}
