//go:build server

package file

import (
	"context"
	"fmt"
	"uuid"

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
	id := uuid.NewV7()
	record := &servermodels.File{
		ID:              id.String(),
		OrganizationID:  identity.Organization.ID,
		CreatedByUserID: identity.User.ID,
		Purpose:         string(input.Purpose),
		StorageBackend:  string(backend),
		StorageKey:      storageKey(identity.Organization.ID, id.String(), input.ContentType),
		OriginalName:    input.FileName,
		ContentType:     input.ContentType,
		ByteSize:        input.ByteSize,
		Status:          string(domain.FileStatusPending),
	}
	// 过期时间统一使用数据库时钟，与后续 expires_at > now() 的比较保持同源。
	if _, err := a.db.NewInsert().Model(record).
		Value("expires_at", "now() + make_interval(secs => ?)", temporaryFileLifetime.Seconds()).
		Returning("expires_at").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create file upload: %w", err)
	}
	return record, nil
}

// storageKey 返回以文件编号命名的存储键。
func storageKey(organizationID, fileID, contentType string) string {
	return "organizations/" + organizationID + "/files/" + fileID + avatarFileExtensions[contentType]
}

// validIdentity 判断用户企业关系是否有效。
func validIdentity(identity *servermodels.Identity) bool {
	return identity != nil && common.ValidUUID(identity.Organization.ID) && common.ValidUUID(identity.User.ID) && identity.User.OrganizationID == identity.Organization.ID
}
