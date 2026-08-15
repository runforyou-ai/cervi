//go:build !server && !ios && !android

package desktop

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	sqliteinfra "github.com/runforyou-ai/cervi/internal/storage/internal/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded desktop migrations: %w", err)
	}

	applied, err := sqliteinfra.Migrate(ctx, db, migrations)
	if err != nil {
		return err
	}
	if applied == 0 {
		slog.Info("桌面端数据库结构已是最新版本")
		return nil
	}
	slog.Info("桌面端数据库迁移完成", "applied", applied)
	return nil
}
