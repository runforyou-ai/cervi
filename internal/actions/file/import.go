//go:build server

package file

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// StorageBackendResolver 返回企业新文件当前应写入的存储类型。
type StorageBackendResolver func(context.Context, string) (domain.FileStorageBackend, error)

// ContentWriter 按文件记录中确定的存储类型写入内容。
type ContentWriter interface {
	Save(context.Context, *servermodels.File, []byte) (string, error)
}

// ImportInput 定义服务端导入联系人头像所需事实。
type ImportInput struct {
	OrganizationID  string
	CreatedByUserID string
	FileName        string
	ContentType     string
	Data            []byte
}

// ImportAction 把服务端取得的联系人头像写为可激活的临时文件。
type ImportAction struct {
	db             *bun.DB
	resolveBackend StorageBackendResolver
	writer         ContentWriter
}

// NewImportAction 创建服务端文件导入操作。
func NewImportAction(db *bun.DB, resolveBackend StorageBackendResolver, writer ContentWriter) *ImportAction {
	return &ImportAction{db: db, resolveBackend: resolveBackend, writer: writer}
}

// Execute 先提交临时元数据，再写入内容并标记为已上传。
func (a *ImportAction) Execute(ctx context.Context, input ImportInput) (*servermodels.File, error) {
	if !common.ValidUUID(input.OrganizationID) || !common.ValidUUID(input.CreatedByUserID) {
		return nil, errors.New("invalid imported file owner")
	}
	metadata, fields := normalizeFileInput(UploadInput{
		Purpose: domain.FilePurposeContactAvatar, FileName: input.FileName,
		ContentType: input.ContentType, ByteSize: int64(len(input.Data)),
	}, domain.FilePurposeContactAvatar)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	backend, err := a.resolveBackend(ctx, input.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("resolve imported file storage: %w", err)
	}
	if backend != domain.FileStorageBackendLocal && backend != domain.FileStorageBackendS3 {
		return nil, fmt.Errorf("invalid imported file storage backend %q", backend)
	}
	fileID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate imported file id: %w", err)
	}
	record := &servermodels.File{
		ID: fileID.String(), OrganizationID: input.OrganizationID, CreatedByUserID: input.CreatedByUserID,
		Purpose: string(domain.FilePurposeContactAvatar), StorageBackend: string(backend),
		StorageKey:   storageKey(input.OrganizationID, fileID.String(), metadata.ContentType),
		OriginalName: metadata.FileName, ContentType: metadata.ContentType, ByteSize: metadata.ByteSize,
		Status: string(domain.FileStatusPending),
	}
	// pending 必须先独立提交，确保后续写入成功但数据库操作失败时仍可被清理任务发现。
	if _, err := a.db.NewInsert().Model(record).
		Value("expires_at", "now() + make_interval(secs => ?)", temporaryFileLifetime.Seconds()).
		Returning("expires_at").Exec(ctx); err != nil {
		return nil, fmt.Errorf("create imported file: %w", err)
	}
	etag, err := a.writer.Save(ctx, record, input.Data)
	if err != nil {
		return nil, fmt.Errorf("write imported file: %w", err)
	}
	result, err := a.db.NewUpdate().Model(record).
		Set("status = ?", domain.FileStatusUploaded).
		Set("etag = ?", common.OptionalString(etag)).
		Set("uploaded_at = now()").
		Set("expires_at = now() + make_interval(secs => ?)", temporaryFileLifetime.Seconds()).
		Set("updated_at = now()").
		Where("f.id = ?", record.ID).
		Where("f.organization_id = ?", record.OrganizationID).
		Where("f.status = ?", domain.FileStatusPending).
		Where("f.expires_at > now()").
		Returning("*").Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark imported file uploaded: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read imported file update count: %w", err)
	}
	if rows == 0 {
		return nil, ErrFileNotFound
	}
	return record, nil
}
