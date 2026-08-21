//go:build server

package appservice

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/filestore"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// CreateFileUpload 创建当前存储开关对应的文件上传请求。
func (b *DirectBackend) CreateFileUpload(ctx context.Context, meta RequestMeta, input FileUploadInput) (FileUpload, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return FileUpload{}, err
	}
	setting, err := b.getS3Setting.Execute(ctx, identity)
	if err != nil {
		return FileUpload{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	backend := domain.FileStorageBackendLocal
	if setting.Enabled {
		backend = domain.FileStorageBackendS3
	}
	record, err := b.createFileUpload.Execute(ctx, identity, backend, fileaction.UploadInput{
		Purpose:     domain.FilePurpose(input.Purpose),
		FileName:    input.FileName,
		ContentType: input.ContentType,
		ByteSize:    input.ByteSize,
	})
	if err != nil {
		return FileUpload{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	request, err := b.fileUploadRequest(ctx, meta, record, setting)
	if err != nil {
		if cleanupErr := b.createFileUpload.DeletePending(ctx, identity.Organization.ID, record.ID); cleanupErr != nil {
			slog.Warn("清理待上传文件记录失败", "organization_id", identity.Organization.ID, "file_id", record.ID, "error", cleanupErr)
		}
		return FileUpload{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	slog.Info("文件上传已创建", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "file_id", record.ID, "storage_backend", backend)
	return FileUpload{File: fileFromModel(record), Request: request}, nil
}

// CompleteFileUpload 核验文件内容并将上传标记为完成。
func (b *DirectBackend) CompleteFileUpload(ctx context.Context, meta RequestMeta, fileID string) (File, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return File{}, err
	}
	record, err := b.getFile.Execute(ctx, identity, fileID)
	if err != nil {
		return File{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed)
	}
	if record.Status == string(domain.FileStatusActive) {
		return fileFromModel(record), nil
	}
	if record.Status == string(domain.FileStatusUploaded) {
		if record.ExpiresAt != nil && record.ExpiresAt.After(time.Now().UTC()) {
			return fileFromModel(record), nil
		}
		return File{}, b.fileOperationError(ctx, meta, fileaction.ErrFileNotFound, cervii18n.ErrorFileUploadCompleteFailed)
	}
	if record.Status != string(domain.FileStatusPending) || record.ExpiresAt == nil || !record.ExpiresAt.After(time.Now().UTC()) {
		return File{}, b.fileOperationError(ctx, meta, fileaction.ErrFileNotFound, cervii18n.ErrorFileUploadCompleteFailed)
	}
	etag, actualSize, err := b.statFile(ctx, identity, record)
	if err != nil || actualSize != record.ByteSize {
		if err == nil {
			err = fmt.Errorf("uploaded file size = %d, want %d", actualSize, record.ByteSize)
		}
		return File{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed)
	}
	record, err = b.markFileUploaded.Execute(ctx, identity, record.ID, etag)
	if err != nil {
		return File{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed)
	}
	slog.Info("文件上传已完成", "organization_id", identity.Organization.ID, "file_id", record.ID, "storage_backend", record.StorageBackend)
	return fileFromModel(record), nil
}

// fileUploadRequest 返回本地上传地址或 S3 预签名请求。
func (b *DirectBackend) fileUploadRequest(ctx context.Context, meta RequestMeta, record *servermodels.File, setting settingaction.S3Setting) (FileUploadRequest, error) {
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		return FileUploadRequest{
			Method: http.MethodPut, URL: fileContentURL(record.ID),
			Headers: map[string]string{"Authorization": "Bearer " + meta.Token, "Content-Type": record.ContentType},
		}, nil
	}
	signed, err := filestore.PresignPut(ctx, s3FileConfig(setting), record.StorageKey, record.ContentType)
	if err != nil {
		return FileUploadRequest{}, fmt.Errorf("presign S3 file upload: %w", err)
	}
	return FileUploadRequest{Method: signed.Method, URL: signed.URL, Headers: signed.Headers}, nil
}

// statFile 核验文件在其原始存储位置中的元数据。
func (b *DirectBackend) statFile(ctx context.Context, identity *servermodels.Identity, record *servermodels.File) (string, int64, error) {
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		info, err := b.localFiles.Stat(record.StorageKey)
		if err != nil {
			return "", 0, fmt.Errorf("stat local file: %w", err)
		}
		return "", info.Size(), nil
	}
	setting, err := b.getS3Setting.Execute(ctx, identity)
	if err != nil {
		return "", 0, err
	}
	info, err := filestore.Stat(ctx, s3FileConfig(setting), record.StorageKey)
	if err != nil {
		return "", 0, fmt.Errorf("stat S3 file: %w", err)
	}
	return info.ETag, info.ByteSize, nil
}

// fileOperationError 转换文件校验和操作错误。
func (b *DirectBackend) fileOperationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, fileFieldKeys(validationError.Fields))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, fileaction.ErrFileNotFound) {
		return NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}
	slog.Warn("文件操作失败", "failure", failureKey, "error", err)
	return FailedError(meta, failureKey)
}

// fileFromModel 把存储文件转换为应用契约。
func fileFromModel(record *servermodels.File) File {
	return File{ID: record.ID, Name: record.OriginalName, ContentType: record.ContentType, ByteSize: record.ByteSize, ContentURL: fileContentURL(record.ID)}
}

// fileFieldKeys 返回文件字段校验文案。
func fileFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		fileaction.ValidationFileNameRequired:   cervii18n.FieldFileNameRequired,
		fileaction.ValidationContentTypeInvalid: cervii18n.FieldFileContentTypeInvalid,
		fileaction.ValidationByteSizeInvalid:    cervii18n.FieldFileByteSizeInvalid,
		fileaction.ValidationPurposeInvalid:     cervii18n.FieldFilePurposeInvalid,
	}
	return translateValidationFields(fields, keys)
}

// s3FileConfig 转换文件存储使用的 S3 配置。
func s3FileConfig(setting settingaction.S3Setting) filestore.S3Config {
	return filestore.S3Config{
		Endpoint: setting.Endpoint, Region: setting.Region, Bucket: setting.Bucket,
		AccessKeyID: setting.AccessKeyID, SecretAccessKey: setting.SecretAccessKey, ForcePathStyle: setting.ForcePathStyle,
	}
}
