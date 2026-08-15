// Package mobile 管理移动端的 SQLite 存储。
package mobile

import (
	"context"
	"fmt"

	sqliteinfra "github.com/runforyou-ai/cervi/internal/storage/internal/sqlite"
	"github.com/uptrace/bun"
)

// Store 管理移动端的 Bun 数据库连接。
type Store struct {
	db *bun.DB
}

// Open 创建移动端 SQLite 数据库并执行移动端迁移。
func Open(ctx context.Context, databasePath string) (*Store, error) {
	db, err := sqliteinfra.Open(ctx, databasePath)
	if err != nil {
		return nil, fmt.Errorf("initialize mobile SQLite: %w", err)
	}
	if err := migrate(ctx, db.DB); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate mobile SQLite: %w", err)
	}

	return &Store{db: db}, nil
}

// Close 关闭移动端 SQLite 数据库连接。
func (s *Store) Close() error {
	return s.db.Close()
}
