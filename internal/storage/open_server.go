//go:build server

package storage

import (
	"context"
	"fmt"
	"log/slog"

	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
)

// Open 初始化 server 构建使用的 PostgreSQL 存储。
func Open(ctx context.Context) (Storage, error) {
	slog.Info("正在初始化 PostgreSQL 存储")

	config, err := serverstorage.ConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load PostgreSQL configuration: %w", err)
	}

	return serverstorage.Open(ctx, config)
}
