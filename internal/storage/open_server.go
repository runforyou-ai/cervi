//go:build server

package storage

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/storage/postgres"
)

// Open initializes the PostgreSQL implementation used by server builds.
func Open(ctx context.Context) (Storage, error) {
	config, err := postgres.ConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load PostgreSQL configuration: %w", err)
	}

	storage, err := postgres.Open(ctx, config)
	if err != nil {
		return nil, err
	}

	return storage, nil
}
