//go:build server

package filecleanup

import (
	"context"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/filestore"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestDeleteLocalFile 验证清理服务删除文件记录指向的本地内容。
func TestDeleteLocalFile(t *testing.T) {
	t.Setenv("FILE_STORAGE_PATH", t.TempDir())
	local, err := filestore.NewLocalStoreFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	key := "organizations/org/files/file.png"
	if err := local.Save(context.Background(), key, strings.NewReader("avatar"), 6); err != nil {
		t.Fatal(err)
	}
	service := &Service{local: local}
	if err := service.delete(context.Background(), &servermodels.File{StorageBackend: string(domain.FileStorageBackendLocal), StorageKey: key}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := local.Stat(key); err == nil {
		t.Fatal("local file still exists")
	}
}

// TestDeleteRejectsUnknownBackend 验证清理服务不会忽略未知存储位置。
func TestDeleteRejectsUnknownBackend(t *testing.T) {
	service := &Service{}
	if err := service.delete(context.Background(), &servermodels.File{StorageBackend: "unknown"}, nil); err == nil {
		t.Fatal("unknown backend was accepted")
	}
}
