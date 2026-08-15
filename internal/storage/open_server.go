//go:build server

package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/runforyou-ai/cervi/internal/storage/postgres"
)

// Open 初始化 server 构建使用的 PostgreSQL 存储。
func Open(ctx context.Context) (Storage, error) {
	slog.Info("正在初始化 PostgreSQL 存储")

	config, err := postgres.ConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load PostgreSQL configuration: %w", err)
	}

	storage, err := postgres.Open(ctx, config)
	if err != nil {
		return nil, err
	}

	return storage, nil
}
