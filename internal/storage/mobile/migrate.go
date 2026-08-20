//go:build !server && (ios || android)

package mobile

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded mobile migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations)
	if err != nil {
		return fmt.Errorf("create mobile migration provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return nil
	}
	slog.Info("移动端数据库迁移完成", "applied", len(results))
	return nil
}
