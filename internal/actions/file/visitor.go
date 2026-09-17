//go:build server

package file

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// VisitorUploadInput 定义渠道访客待上传文件的归属与元数据。
type VisitorUploadInput struct {
	OrganizationID    string
	CreatedByUserID   string
	ChannelIdentityID string
	Upload            UploadInput
}

// CreateVisitorPending 在业务事务中创建渠道访客上传的临时文件，访客不使用分片上传。
func CreateVisitorPending(ctx context.Context, db bun.IDB, backend domain.FileStorageBackend, input VisitorUploadInput) (*servermodels.File, error) {
	upload, fields := NormalizeUploadInput(input.Upload)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	record, err := pendingFile(input.OrganizationID, input.CreatedByUserID, backend, upload, 0)
	if err != nil {
		return nil, err
	}
	record.UploaderChannelIdentityID = &input.ChannelIdentityID
	return record, insertPendingFile(ctx, db, record)
}

// LoadVisitorUpload 读取该渠道访客上传且尚未过期的文件。
func LoadVisitorUpload(ctx context.Context, db bun.IDB, organizationID, channelIdentityID, fileID string) (*servermodels.File, error) {
	if !common.ValidUUID(fileID) {
		return nil, ErrFileNotFound
	}
	record := &servermodels.File{}
	err := db.NewSelect().Model(record).
		ColumnExpr("f.*").
		ColumnExpr("(f.expires_at IS NULL OR f.expires_at <= now()) AS expired").
		Where("f.id = ? AND f.organization_id = ?", fileID, organizationID).
		Where("f.purpose = ? AND f.uploader_channel_identity_id = ?", domain.FilePurposeMessageAttachment, channelIdentityID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get visitor upload: %w", err)
	}
	return record, nil
}

// CompleteVisitorUpload 按文件当前状态推进访客上传：已激活或已上传且未过期时幂等返回，待上传时核验内容后标记完成。
func CompleteVisitorUpload(ctx context.Context, db bun.IDB, organizationID, channelIdentityID, fileID string, finalize FinalizeFunc) (*servermodels.File, error) {
	record, err := LoadVisitorUpload(ctx, db, organizationID, channelIdentityID, fileID)
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
	etag, actualSize, err := finalize(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("finalize visitor uploaded file: %w", err)
	}
	if actualSize != record.ByteSize {
		return nil, fmt.Errorf("visitor uploaded file size = %d, want %d", actualSize, record.ByteSize)
	}
	return markVisitorUploaded(ctx, db, organizationID, channelIdentityID, fileID, etag)
}

// markVisitorUploaded 保存访客上传的核验结果，状态转移由带 status 和 expires_at 守卫的原子 UPDATE 保证。
func markVisitorUploaded(ctx context.Context, db bun.IDB, organizationID, channelIdentityID, fileID, etag string) (*servermodels.File, error) {
	record := &servermodels.File{}
	result, err := db.NewUpdate().Model(record).
		Set("status = ?", domain.FileStatusUploaded).
		Set("etag = ?", common.OptionalString(strings.TrimSpace(etag))).
		Set("uploaded_at = now()").
		Set("expires_at = now() + make_interval(secs => ?)", temporaryFileLifetime.Seconds()).
		Set("updated_at = now()").
		Where("f.id = ? AND f.organization_id = ?", fileID, organizationID).
		Where("f.uploader_channel_identity_id = ?", channelIdentityID).
		Where("f.status = ?", domain.FileStatusPending).
		Where("f.expires_at > now()").
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark visitor file uploaded: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read marked visitor file count: %w", err)
	}
	if rows == 0 {
		return nil, ErrFileNotFound
	}
	return record, nil
}

// VisitorPendingByStorageKey 按存储键读取指定渠道访客待写入内容的本地文件，企业由文件及其上传渠道身份确定。
func (q *GetQuery) VisitorPendingByStorageKey(ctx context.Context, externalID, storageKey string) (*servermodels.File, error) {
	record := &servermodels.File{}
	err := q.db.NewSelect().Model(record).
		ColumnExpr("f.*").
		ColumnExpr("(f.expires_at IS NULL OR f.expires_at <= now()) AS expired").
		Join("JOIN contact_channel_identities AS cci ON cci.id = f.uploader_channel_identity_id AND cci.organization_id = f.organization_id").
		Where("f.storage_key = ?", storageKey).
		Where("f.purpose = ? AND f.storage_backend = ?", domain.FilePurposeMessageAttachment, domain.FileStorageBackendLocal).
		Where("cci.external_id = ?", externalID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get visitor upload by storage key: %w", err)
	}
	return record, nil
}
