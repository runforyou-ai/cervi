//go:build !server

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

// Migrate 执行指定的 SQLite 迁移并返回已应用数量。
func Migrate(ctx context.Context, db *sql.DB, migrations fs.FS) (int, error) {
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations)
	if err != nil {
		return 0, fmt.Errorf("create SQLite migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return 0, err
	}
	return len(results), nil
}
