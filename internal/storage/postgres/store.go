//go:build server

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Store 管理 Bun 数据库连接池。
type Store struct {
	db *bun.DB
}

// Open 连接 PostgreSQL 并执行数据库迁移。
func Open(ctx context.Context, config Config) (*Store, error) {
	startupCtx, cancel := context.WithTimeout(ctx, config.StartupTimeout)
	defer cancel()
	if config.MaxOpenConns == 0 {
		slog.Warn("PostgreSQL 最大连接数未设置限制")
	}

	sqlDB := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(config.DSN)))
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	db := bun.NewDB(sqlDB, pgdialect.New())
	if err := db.PingContext(startupCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	slog.Info("PostgreSQL 连接成功")

	if err := migrate(startupCtx, sqlDB); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate PostgreSQL: %w", err)
	}

	return &Store{db: db}, nil
}

// Close 关闭 PostgreSQL 连接池。
func (s *Store) Close() error {
	return s.db.Close()
}
