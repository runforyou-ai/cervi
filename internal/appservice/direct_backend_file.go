//go:build server

package appservice

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/actions/filemaintenance"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// fileOps 持有文件存储的 Action、Query 和部署级存储配置。
type fileOps struct {
	createFileUpload   *fileaction.CreateUploadAction
	cancelFileUpload   *filemaintenance.CancelUploadAction
	completeFileUpload *fileaction.CompleteUploadAction
	getFile            *fileaction.GetQuery
	localFiles         *serverfilecontent.LocalStore
	s3                 serverfilecontent.S3Config
}

// newFileOps 创建文件存储的业务实现依赖。
func newFileOps(db *bun.DB, localFiles *serverfilecontent.LocalStore, s3 serverfilecontent.S3Config) fileOps {
	return fileOps{
		createFileUpload:   fileaction.NewCreateUploadAction(db),
		cancelFileUpload:   filemaintenance.NewCancelUploadAction(db),
		completeFileUpload: fileaction.NewCompleteUploadAction(db),
		getFile:            fileaction.NewGetQuery(db),
		localFiles:         localFiles,
		s3:                 s3,
	}
}

// CreateFileUpload 创建当前存储开关对应的文件上传请求。
func (o *directOperations) CreateFileUpload(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input FileUploadInput) (FileUpload, error) {
	backend := domain.FileStorageBackendLocal
	if o.s3.Enabled {
		backend = domain.FileStorageBackendS3
	}
	record, err := o.createFileUpload.Execute(ctx, identity, backend, fileaction.UploadInput{
		Purpose:     domain.FilePurpose(input.Purpose),
		FileName:    input.FileName,
		ContentType: input.ContentType,
		ByteSize:    input.ByteSize,
	})
	if err != nil {
		return FileUpload{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	return o.prepareFileUpload(ctx, meta, identity, record)
}

// PrepareFileUpload 在实际开始传输时取得上传请求，并复用已创建的分片会话。
func (o *directOperations) PrepareFileUpload(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, fileID string) (FileUpload, error) {
	record, err := o.getFile.Execute(ctx, identity, fileID)
	if err == nil && (record.CreatedByUserID != identity.User.ID || record.Status != string(domain.FileStatusPending) || record.Expired) {
		err = fileaction.ErrFileNotFound
	}
	if err != nil {
		return FileUpload{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	return o.prepareFileUpload(ctx, meta, identity, record)
}

// prepareFileUpload 为已解析的文件位置准备普通上传请求或分片会话。
func (o *directOperations) prepareFileUpload(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, record *servermodels.File) (FileUpload, error) {
	contentURL, err := fileContentURL(domain.FileStorageBackend(record.StorageBackend), record.StorageKey, o.s3.PublicBaseURL)
	if err != nil {
		return FileUpload{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	if record.PartSize > 0 {
		if record.StorageBackend == string(domain.FileStorageBackendS3) && record.MultipartUploadID == nil {
			uploadID, err := serverfilecontent.CreateMultipart(ctx, o.s3, record.StorageKey, record.ContentType)
			if err != nil {
				return FileUpload{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
			}
			stored, saveErr := o.createFileUpload.SetMultipartUpload(ctx, identity, record.ID, uploadID)
			if !stored {
				// 并发准备只保留一个会话，未采用的远端会话立即清理。
				if cleanupErr := serverfilecontent.AbortMultipart(context.WithoutCancel(ctx), o.s3, record.StorageKey, uploadID); cleanupErr != nil {
					slog.Warn("清除未保存的分片会话失败", "file_id", record.ID, "error", cleanupErr)
				}
				if saveErr != nil {
					return FileUpload{}, o.fileOperationError(ctx, meta, saveErr, cervii18n.ErrorFileUploadCreateFailed)
				}
				current, err := o.getFile.Execute(ctx, identity, record.ID)
				if err == nil && (current.MultipartUploadID == nil || current.Status != string(domain.FileStatusPending) || current.Expired) {
					err = fileaction.ErrFileNotFound
				}
				if err != nil {
					return FileUpload{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
				}
				record = current
			}
		}
		return FileUpload{File: fileFromModel(record, contentURL), PartSize: record.PartSize}, nil
	}
	request, err := o.fileUploadRequest(ctx, meta, record, contentURL)
	if err != nil {
		return FileUpload{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	return FileUpload{File: fileFromModel(record, contentURL), Request: request}, nil
}

// CompleteFileUpload 核验文件内容并将上传标记为完成。
func (o *directOperations) CompleteFileUpload(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, fileID string) (File, error) {
	record, err := o.completeFileUpload.Execute(ctx, identity, fileID, o.finalizeFileContent)
	if err != nil {
		return File{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed)
	}
	o.cleanupCompletedParts(record)
	return o.completedFile(ctx, meta, record)
}

// completedFile 为已完成的上传生成文件地址并记录结果。
func (o *directOperations) completedFile(ctx context.Context, meta RequestMeta, record *servermodels.File) (File, error) {
	contentURL, err := fileContentURL(domain.FileStorageBackend(record.StorageBackend), record.StorageKey, o.s3.PublicBaseURL)
	if err != nil {
		return File{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed)
	}
	slog.Info("文件上传已完成", "organization_id", record.OrganizationID, "file_id", record.ID, "storage_backend", record.StorageBackend)
	return fileFromModel(record, contentURL), nil
}

// fileUploadRequest 返回本地上传地址或 S3 预签名请求。
func (o *directOperations) fileUploadRequest(ctx context.Context, meta RequestMeta, record *servermodels.File, contentURL string) (FileUploadRequest, error) {
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		return FileUploadRequest{
			Method: http.MethodPut, URL: contentURL,
			Headers: map[string]string{"Authorization": "Bearer " + meta.Token, "Content-Type": record.ContentType},
		}, nil
	}
	signed, err := serverfilecontent.PresignPut(ctx, o.s3, record.StorageKey, record.ContentType)
	if err != nil {
		return FileUploadRequest{}, fmt.Errorf("presign S3 file upload: %w", err)
	}
	return FileUploadRequest{Method: signed.Method, URL: signed.URL, Headers: signed.Headers}, nil
}

// statFile 按文件记录的存储类型核验内容。
func (o *directOperations) statFile(ctx context.Context, record *servermodels.File) (string, int64, error) {
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		info, err := o.localFiles.Stat(ctx, record.StorageKey)
		if err != nil {
			return "", 0, fmt.Errorf("stat local file: %w", err)
		}
		return "", info.Size(), nil
	}
	info, err := serverfilecontent.Stat(ctx, o.s3, record.StorageKey)
	if err != nil {
		return "", 0, fmt.Errorf("stat S3 file: %w", err)
	}
	return info.ETag, info.ByteSize, nil
}

// fileOperationError 转换文件校验和操作错误。
func (o *directOperations) fileOperationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		// 映射文件字段校验文案。
		keys := map[common.FieldCode]cervii18n.Key{
			fileaction.ValidationFileNameRequired:   cervii18n.FieldFileNameRequired,
			fileaction.ValidationContentTypeInvalid: cervii18n.FieldFileContentTypeInvalid,
			fileaction.ValidationByteSizeInvalid:    cervii18n.FieldFileByteSizeInvalid,
			fileaction.ValidationDocumentTooLarge:   cervii18n.FieldKnowledgeDocumentTooLarge,
			fileaction.ValidationPurposeInvalid:     cervii18n.FieldFilePurposeInvalid,
		}
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, keys))
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

// cleanupCompletedParts 清除已经合并且已确认完成的本地分片。
func (o *directOperations) cleanupCompletedParts(record *servermodels.File) {
	if record.PartSize > 0 && record.StorageBackend == string(domain.FileStorageBackendLocal) {
		if err := o.localFiles.DeleteParts(record.StorageKey); err != nil {
			slog.Warn("清理已合并分片失败", "file_id", record.ID, "error", err)
		}
	}
}
