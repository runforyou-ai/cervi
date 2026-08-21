//go:build server

package file

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/runforyou-ai/cervi/internal/common"
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
	input, fields := normalizeUploadInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if !validIdentity(identity) {
		return nil, common.ErrIdentityInvalid
	}
	if backend != domain.FileStorageBackendLocal && backend != domain.FileStorageBackendS3 {
		return nil, fmt.Errorf("invalid file storage backend %q", backend)
	}
	id := uuid.NewString()
	record := &servermodels.File{
		ID:              id,
		OrganizationID:  identity.Organization.ID,
		CreatedByUserID: identity.User.ID,
		Purpose:         string(input.Purpose),
		StorageBackend:  string(backend),
		StorageKey:      "organizations/" + identity.Organization.ID + "/files/" + id + "/" + originalObjectName(input),
		OriginalName:    input.FileName,
		ContentType:     input.ContentType,
		ByteSize:        input.ByteSize,
		Status:          string(domain.FileStatusPending),
	}
	if _, err := a.db.NewInsert().Model(record).Exec(ctx); err != nil {
		return nil, fmt.Errorf("create file upload: %w", err)
	}
	return record, nil
}

// DeletePending 删除无法继续上传的待处理文件记录。
func (a *CreateUploadAction) DeletePending(ctx context.Context, organizationID, fileID string) error {
	_, err := a.db.NewDelete().
		Model((*servermodels.File)(nil)).
		Where("organization_id = ?", organizationID).
		Where("id = ?", fileID).
		Where("status = ?", domain.FileStatusPending).
		Exec(ctx)
	return err
}

// validIdentity 判断用户企业关系是否有效。
func validIdentity(identity *servermodels.Identity) bool {
	return identity != nil && common.ValidUUID(identity.Organization.ID) && common.ValidUUID(identity.User.ID) && identity.User.OrganizationID == identity.Organization.ID
}
