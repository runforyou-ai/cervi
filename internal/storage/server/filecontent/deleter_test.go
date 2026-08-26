//go:build server

package filecontent

import (
	"context"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestDeleterDeleteLocalFile 验证删除器移除本地文件。
func TestDeleterDeleteLocalFile(t *testing.T) {
	local := &LocalStore{root: t.TempDir()}
	key := "organizations/org/files/file.png"
	if err := local.Save(context.Background(), key, strings.NewReader("avatar"), 6); err != nil {
		t.Fatal(err)
	}
	deleter := NewDeleter(local, nil)
	if err := deleter.Delete(context.Background(), &servermodels.File{
		StorageBackend: string(domain.FileStorageBackendLocal), StorageKey: key,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := local.Stat(context.Background(), key); err == nil {
		t.Fatal("local file still exists")
	}
}

// TestDeleterRejectsUnknownBackend 验证删除器拒绝未知存储类型。
func TestDeleterRejectsUnknownBackend(t *testing.T) {
	deleter := NewDeleter(nil, nil)
	if err := deleter.Delete(context.Background(), &servermodels.File{StorageBackend: "unknown"}); err == nil {
		t.Fatal("unknown backend was accepted")
	}
}
