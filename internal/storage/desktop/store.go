//go:build !server && !ios && !android

// Package desktop 管理桌面端的 SQLite 存储。
package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

const sqliteDriverName = "sqlite3"

// Store 管理桌面端的 Bun 数据库连接。
type Store struct {
	db *bun.DB
}

// Open 创建桌面端 SQLite 数据库并执行桌面端迁移。
func Open(ctx context.Context, databasePath string) (*Store, error) {
	db, err := openSQLite(ctx, databasePath)
	if err != nil {
		return nil, fmt.Errorf("initialize desktop SQLite: %w", err)
	}
	slog.Info("桌面端 SQLite 连接成功")

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

func openSQLite(ctx context.Context, databasePath string) (*bun.DB, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		return nil, fmt.Errorf("create SQLite data directory: %w", err)
	}

	sqlDB, err := sql.Open(sqliteDriverName, sqliteDataSourceName(databasePath))
	if err != nil {
		return nil, fmt.Errorf("open SQLite: %w", err)
	}

	db := bun.NewDB(sqlDB, sqlitedialect.New())
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to SQLite: %w", err)
	}
	return db, nil
}

func sqliteDataSourceName(databasePath string) string {
	query := url.Values{
		"_busy_timeout": {"5000"},
		"_foreign_keys": {"on"},
		"_journal_mode": {"WAL"},
		"mode":          {"rwc"},
	}
	return databasePath + "?" + query.Encode()
}
