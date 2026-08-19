//go:build server

package storage

import (
	"context"
	"fmt"

	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
)

// Open 初始化服务端使用的 PostgreSQL 存储。
func Open(ctx context.Context) (*serverstorage.Store, error) {
	config, err := serverstorage.ConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load PostgreSQL configuration: %w", err)
	}

	return serverstorage.Open(ctx, config)
}
