//go:build !server && !ios && !android

package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	desktopstorage "github.com/runforyou-ai/cervi/internal/storage/desktop"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const desktopDatabaseName = "cervi-desktop.db"

// Open 初始化桌面端使用的 SQLite 存储。
func Open(ctx context.Context) (*desktopstorage.Store, error) {
	dataDirectory := strings.TrimSpace(os.Getenv("DESKTOP_DATA_DIR"))
	if dataDirectory == "" {
		dataDirectory = filepath.Join(application.Path(application.PathDataHome), "cervi")
	}
	absoluteDirectory, err := filepath.Abs(dataDirectory)
	if err != nil {
		return nil, fmt.Errorf("resolve desktop data directory: %w", err)
	}
	databasePath := filepath.Join(absoluteDirectory, desktopDatabaseName)
	return desktopstorage.Open(ctx, databasePath)
}
