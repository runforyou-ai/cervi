//go:build server

package filecontent

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// S3ConfigResolver 返回企业的对象存储配置。
type S3ConfigResolver func(context.Context, string) (S3Config, error)

// Deleter 删除文件记录指向的内容。
type Deleter struct {
	local     *LocalStore
	resolveS3 S3ConfigResolver
}

// NewDeleter 创建文件内容删除器。
func NewDeleter(local *LocalStore, resolveS3 S3ConfigResolver) *Deleter {
	return &Deleter{local: local, resolveS3: resolveS3}
}

// Delete 按文件记录中的存储类型删除内容。
func (d *Deleter) Delete(ctx context.Context, record *servermodels.File) error {
	switch domain.FileStorageBackend(record.StorageBackend) {
	case domain.FileStorageBackendLocal:
		return d.local.Delete(record.StorageKey)
	case domain.FileStorageBackendS3:
		config, err := d.resolveS3(ctx, record.OrganizationID)
		if err != nil {
			return err
		}
		return Delete(ctx, config, record.StorageKey)
	default:
		return fmt.Errorf("invalid file storage backend %q", record.StorageBackend)
	}
}
