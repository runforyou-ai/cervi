//go:build !server && (ios || android)

package storage

import (
	"context"
	"log/slog"
	"path/filepath"

	mobilestorage "github.com/runforyou-ai/cervi/internal/storage/mobile"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const mobileDatabaseName = "cervi-mobile.db"

// Open 初始化移动端使用的 SQLite 存储。
func Open(ctx context.Context) (Storage, error) {
	databasePath := filepath.Join(application.Mobile.StoragePath(), mobileDatabaseName)
	slog.Info("正在初始化移动端 SQLite 存储", "path", databasePath)

	return mobilestorage.Open(ctx, databasePath)
}
