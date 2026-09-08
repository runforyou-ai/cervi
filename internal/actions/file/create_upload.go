//go:build server

package file

import (
	"context"
	"fmt"
	"uuid"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateUploadAction 创建待上传文件记录。
type CreateUploadAction struct {
	db *bun.DB
}

// NewCreateUploadAction 创建文件上传操作。
func NewCreateUploadAction(db *bun.DB) *CreateUploadAction {
	return &CreateUploadAction{db: db}
}

// Execute 校验元数据并创建指定存储位置的待上传文件。
func (a *CreateUploadAction) Execute(ctx context.Context, identity *servermodels.Identity, backend domain.FileStorageBackend, input UploadInput) (*servermodels.File, error) {
	input, fields := NormalizeUploadInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var record *servermodels.File
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		var err error
		record, err = CreatePending(ctx, tx, identity, backend, input)
		return err
	})
	return record, err
}

// CreatePending 使用已规范化的元数据在业务事务中创建文件记录。
func CreatePending(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, backend domain.FileStorageBackend, input UploadInput) (*servermodels.File, error) {
	if backend != domain.FileStorageBackendLocal && backend != domain.FileStorageBackendS3 {
		return nil, fmt.Errorf("invalid file storage backend %q", backend)
	}
	id := uuid.NewV7().String()
	partSize := int64(0)
	if input.ByteSize > domain.FilePartSize {
		// 大文件按 S3 最多 10000 片增大片大小。
		partSize = max(domain.FilePartSize, (input.ByteSize-1)/10000+1)
	}
	record := &servermodels.File{ID: id, OrganizationID: identity.Organization.ID, CreatedByUserID: identity.User.ID,
		Purpose: string(input.Purpose), StorageBackend: string(backend), StorageKey: storageKey(identity.Organization.ID, id, input.ContentType),
		OriginalName: input.FileName, ContentType: input.ContentType, ByteSize: input.ByteSize, PartSize: partSize, Status: string(domain.FileStatusPending)}
	_, err := tx.NewInsert().Model(record).Value("expires_at", "now() + make_interval(secs => ?)", temporaryFileLifetime.Seconds()).Returning("expires_at").Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("create file upload: %w", err)
	}
	return record, nil
}

// storageKey 返回以文件编号命名的存储键。
func storageKey(organizationID, fileID, contentType string) string {
	extension := imageFileExtensions[contentType]
	if extension == "" {
		extension = ".bin"
	}
	return "organizations/" + organizationID + "/files/" + fileID + extension
}
