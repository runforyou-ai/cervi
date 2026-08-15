//go:build !server && !ios && !android

// Package desktop 管理桌面端的 SQLite 存储。
package desktop

import (
	"context"
	"fmt"

	sqliteinfra "github.com/runforyou-ai/cervi/internal/storage/internal/sqlite"
	"github.com/uptrace/bun"
)

// Store 管理桌面端的 Bun 数据库连接。
type Store struct {
	db *bun.DB
}

// Open 创建桌面端 SQLite 数据库并执行桌面端迁移。
func Open(ctx context.Context, databasePath string) (*Store, error) {
	db, err := sqliteinfra.Open(ctx, databasePath)
	if err != nil {
		return nil, fmt.Errorf("initialize desktop SQLite: %w", err)
	}
	if err := migrate(ctx, db.DB); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate desktop SQLite: %w", err)
	}

	return &Store{db: db}, nil
}

// Close 关闭桌面端 SQLite 数据库连接。
func (s *Store) Close() error {
	return s.db.Close()
}
