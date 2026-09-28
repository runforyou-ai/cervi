//go:build server

package server

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrate 检查待执行迁移，并在 Goose 迁移锁内执行 PostgreSQL 数据库迁移。
func migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	// 抢锁失败时每秒重试一次，最长等待 5 分钟。
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(1, 300))
	if err != nil {
		return fmt.Errorf("create migration lock: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrations,
		goose.WithSessionLocker(locker),
		goose.WithAllowOutofOrder(true),
	)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	// 无锁检查到迁移已全部应用时直接返回；版本表尚未建立时检查报错，由加锁的 Up 建表并迁移。
	if pending, err := provider.HasPending(ctx); err == nil && !pending {
		return nil
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return nil
	}
	slog.Info("PostgreSQL 数据库迁移完成", "applied", len(results))
	return nil
}
