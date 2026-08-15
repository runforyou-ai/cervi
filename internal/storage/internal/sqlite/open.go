// Package sqlite 提供桌面端和移动端共用的 SQLite 基础能力。
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

const driverName = "sqlite3"

// Open 创建 SQLite 数据库连接并绑定 Bun SQLite 方言。
func Open(ctx context.Context, databasePath string) (*bun.DB, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		return nil, fmt.Errorf("create SQLite data directory: %w", err)
	}

	sqlDB, err := sql.Open(driverName, dataSourceName(databasePath))
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

func dataSourceName(databasePath string) string {
	query := url.Values{
		"_busy_timeout": {"5000"},
		"_foreign_keys": {"on"},
		"_journal_mode": {"WAL"},
		"mode":          {"rwc"},
	}
	return (&url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(databasePath),
		RawQuery: query.Encode(),
	}).String()
}
