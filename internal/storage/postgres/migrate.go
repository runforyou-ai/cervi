//go:build server

package postgres

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

// migrationFiles 保存内嵌的数据库迁移文件。
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrate 在 PostgreSQL 会话锁保护下执行待应用迁移。
func migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create migration lock: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrations,
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		slog.Info("数据库结构已是最新版本")
		return nil
	}
	slog.Info("数据库迁移完成", "applied", len(results))
	return nil
}
