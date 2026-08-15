package mobile

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	sqliteinfra "github.com/runforyou-ai/cervi/internal/storage/internal/sqlite"
)

// migrationFiles 保存移动端内嵌的数据库迁移文件。
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrate 执行移动端待应用的数据库迁移。
func migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded mobile migrations: %w", err)
	}

	applied, err := sqliteinfra.Migrate(ctx, db, migrations)
	if err != nil {
		return err
	}
	if applied == 0 {
		slog.Info("移动端数据库结构已是最新版本")
		return nil
	}
	slog.Info("移动端数据库迁移完成", "applied", applied)
	return nil
}
