//go:build server

package file

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// StatFunc 按文件记录的存储类型核验已上传内容，返回 ETag 和实际字节数。
type StatFunc func(ctx context.Context, record *servermodels.File) (etag string, byteSize int64, err error)

// CompleteUploadAction 核验文件内容并将上传标记为完成。
type CompleteUploadAction struct {
	db *bun.DB
}

// NewCompleteUploadAction 创建文件上传完成操作。
func NewCompleteUploadAction(db *bun.DB) *CompleteUploadAction {
	return &CompleteUploadAction{db: db}
}

// Execute 按文件当前状态推进上传流程：已激活或已上传且未过期时幂等返回，待上传时核验内容后标记完成。
func (a *CompleteUploadAction) Execute(ctx context.Context, identity *servermodels.Identity, fileID string, stat StatFunc) (*servermodels.File, error) {
	if !validIdentity(identity) {
		return nil, common.ErrIdentityInvalid
	}
	record, err := get(ctx, a.db, identity.Organization.ID, fileID, "")
	if err != nil {
		return nil, err
	}
	switch record.Status {
	case string(domain.FileStatusActive):
		return record, nil
	case string(domain.FileStatusUploaded):
		if record.Expired {
			return nil, ErrFileNotFound
		}
		return record, nil
	case string(domain.FileStatusPending):
		if record.Expired {
			return nil, ErrFileNotFound
		}
	default:
		return nil, ErrFileNotFound
	}
	etag, actualSize, err := stat(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("stat uploaded file: %w", err)
	}
	if actualSize != record.ByteSize {
		return nil, fmt.Errorf("uploaded file size = %d, want %d", actualSize, record.ByteSize)
	}
	// 最终状态转移由带 status 和 expires_at 守卫的原子 UPDATE 保证，并发下重复核验只会有一次生效。
	return NewMarkUploadedAction(a.db).Execute(ctx, identity, record.ID, etag)
}
