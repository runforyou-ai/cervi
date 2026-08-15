package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Storage owns the Bun database handle used by the server process.
type Storage struct {
	db               *bun.DB
	migrationVersion int64
}

// Open connects to PostgreSQL, verifies the connection, and applies all
// embedded Goose migrations before returning.
func Open(ctx context.Context, config Config) (*Storage, error) {
	startupCtx, cancel := context.WithTimeout(ctx, config.StartupTimeout)
	defer cancel()

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

	version, err := migrate(startupCtx, sqlDB)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate PostgreSQL: %w", err)
	}

	return &Storage{db: db, migrationVersion: version}, nil
}

// DB exposes the Bun handle for repositories in higher layers.
func (s *Storage) DB() *bun.DB {
	return s.db
}

// MigrationVersion is the current Goose schema version after initialization.
func (s *Storage) MigrationVersion() int64 {
	return s.migrationVersion
}

// Close releases the PostgreSQL connection pool.
func (s *Storage) Close() error {
	return s.db.Close()
}
