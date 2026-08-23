//go:build server

package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"

	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Store 管理 Bun 数据库连接池。
type Store struct {
	db *bun.DB
}

// Open 连接 PostgreSQL 并执行数据库迁移。
func Open(ctx context.Context, config serverconfig.DatabaseConfig) (*Store, error) {
	if config.MaxOpenConnections == 0 {
		slog.Warn("PostgreSQL 最大连接数未设置限制")
	}

	sqlDB := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithDSN(postgresDSN(config)),
		pgdriver.WithConnParams(map[string]any{"timezone": "UTC"}),
	))
	sqlDB.SetMaxOpenConns(config.MaxOpenConnections)
	sqlDB.SetMaxIdleConns(config.MaxIdleConnections)
	sqlDB.SetConnMaxLifetime(config.ConnectionMaxLifetime.Value())
	sqlDB.SetConnMaxIdleTime(config.ConnectionMaxIdleTime.Value())

	db := bun.NewDB(sqlDB, pgdialect.New())
	connectCtx, cancelConnect := context.WithTimeout(ctx, config.ConnectTimeout.Value())
	connectErr := sqlDB.PingContext(connectCtx)
	cancelConnect()
	if connectErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", connectErr)
	}
	slog.Info("PostgreSQL 连接成功",
		"timezone", "UTC",
		"max_open_connections", config.MaxOpenConnections,
		"max_idle_connections", config.MaxIdleConnections,
	)

	migrationCtx, cancelMigration := context.WithTimeout(ctx, config.MigrationTimeout.Value())
	migrationErr := migrate(migrationCtx, sqlDB)
	cancelMigration()
	if migrationErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate PostgreSQL: %w", migrationErr)
	}

	return &Store{db: db}, nil
}

// postgresDSN 将分项配置编码为 PostgreSQL 驱动连接地址。
func postgresDSN(config serverconfig.DatabaseConfig) string {
	databaseURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.User, config.Password),
		Host:   net.JoinHostPort(config.Host, strconv.Itoa(config.Port)),
		Path:   config.Name,
	}
	query := databaseURL.Query()
	query.Set("sslmode", config.SSLMode)
	databaseURL.RawQuery = query.Encode()
	return databaseURL.String()
}

// Close 关闭 PostgreSQL 连接池。
func (s *Store) Close() error {
	return s.db.Close()
}

// DB 返回服务端使用的 Bun 数据库连接。
func (s *Store) DB() *bun.DB {
	return s.db
}
