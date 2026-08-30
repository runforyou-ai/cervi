//go:build server

package appservice

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
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
	contentURL, err := fileContentURL(domain.FileStorageBackend(record.StorageBackend), record.StorageKey, setting.PublicBaseURL)
	if err != nil {
		return FileUpload{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	request, err := b.fileUploadRequest(ctx, meta, record, setting, contentURL)
	if err != nil {
		return FileUpload{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	return FileUpload{File: fileFromModel(record, contentURL), Request: request}, nil
}

// CompleteFileUpload 核验文件内容并将上传标记为完成。
func (b *DirectBackend) CompleteFileUpload(ctx context.Context, meta RequestMeta, fileID string) (File, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return File{}, err
	}
	record, err := b.completeFileUpload.Execute(ctx, identity, fileID, b.statFile)
	if err != nil {
		return File{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed)
	}
	contentURL, err := b.fileURLForRecord(ctx, record)
	if err != nil {
		return File{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed)
	}
	slog.Info("文件上传已完成", "organization_id", identity.Organization.ID, "file_id", record.ID, "storage_backend", record.StorageBackend)
	return fileFromModel(record, contentURL), nil
}

// fileUploadRequest 返回本地上传地址或 S3 预签名请求。
func (b *DirectBackend) fileUploadRequest(ctx context.Context, meta RequestMeta, record *servermodels.File, setting settingaction.S3Setting, contentURL string) (FileUploadRequest, error) {
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		return FileUploadRequest{
			Method: http.MethodPut, URL: contentURL,
			Headers: map[string]string{"Authorization": "Bearer " + meta.Token, "Content-Type": record.ContentType},
		}, nil
	}
	signed, err := serverfilecontent.PresignPut(ctx, s3FileConfig(setting), record.StorageKey, record.ContentType)
	if err != nil {
		return FileUploadRequest{}, fmt.Errorf("presign S3 file upload: %w", err)
	}
	return FileUploadRequest{Method: signed.Method, URL: signed.URL, Headers: signed.Headers}, nil
}

// fileURLForRecord 按文件记录和所属企业设置生成公开地址。
func (b *DirectBackend) fileURLForRecord(ctx context.Context, record *servermodels.File) (string, error) {
	publicBaseURL := ""
	if record.StorageBackend == string(domain.FileStorageBackendS3) {
		setting, err := b.getS3Setting.ExecuteForOrganization(ctx, record.OrganizationID)
		if err != nil {
			return "", err
		}
		publicBaseURL = setting.PublicBaseURL
	}
	return fileContentURL(domain.FileStorageBackend(record.StorageBackend), record.StorageKey, publicBaseURL)
}

// statFile 按文件记录的存储类型核验内容。
func (b *DirectBackend) statFile(ctx context.Context, record *servermodels.File) (string, int64, error) {
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		info, err := b.localFiles.Stat(ctx, record.StorageKey)
		if err != nil {
			return "", 0, fmt.Errorf("stat local file: %w", err)
		}
		return "", info.Size(), nil
	}
	setting, err := b.getS3Setting.ExecuteForOrganization(ctx, record.OrganizationID)
	if err != nil {
		return "", 0, err
	}
	info, err := serverfilecontent.Stat(ctx, s3FileConfig(setting), record.StorageKey)
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
func fileFromModel(record *servermodels.File, contentURL string) File {
	return File{ID: record.ID, Name: record.OriginalName, ContentType: record.ContentType, ByteSize: record.ByteSize, ContentURL: contentURL}
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
func s3FileConfig(setting settingaction.S3Setting) serverfilecontent.S3Config {
	return serverfilecontent.S3Config{
		Endpoint: setting.Endpoint, Region: setting.Region, Bucket: setting.Bucket,
		AccessKeyID: setting.AccessKeyID, SecretAccessKey: setting.SecretAccessKey, ForcePathStyle: setting.ForcePathStyle,
	}
}
