//go:build !server && !ios && !android

package storage

import (
	"context"
	"log/slog"
	"path/filepath"

	desktopstorage "github.com/runforyou-ai/cervi/internal/storage/desktop"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const desktopDatabaseName = "cervi-desktop.db"

// Open 初始化桌面端使用的 SQLite 存储。
func Open(ctx context.Context) (Storage, error) {
	databasePath := filepath.Join(
		application.Path(application.PathDataHome),
		"cervi",
		desktopDatabaseName,
	)
	slog.Info("正在初始化桌面端 SQLite 存储", "path", databasePath)

	return desktopstorage.Open(ctx, databasePath)
}
