//go:build server

package appservice

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// CreateFilePartUpload 为当前用户的有效临时文件签发分片直传请求。
func (o *directOperations) CreateFilePartUpload(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, fileID string, input FilePartUploadInput) (FileUploadRequest, error) {
	record, err := o.getFile.Execute(ctx, identity, fileID)
	if err == nil && (record.CreatedByUserID != identity.User.ID || record.Status != string(domain.FileStatusPending) || record.Expired) {
		err = fileaction.ErrFileNotFound
	}
	if err != nil {
		return FileUploadRequest{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	size, err := fileaction.UploadPartSize(record, input.PartNumber)
	if err != nil {
		return FileUploadRequest{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		contentURL, err := fileContentURL(domain.FileStorageBackendLocal, record.StorageKey, "")
		if err != nil {
			return FileUploadRequest{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
		}
		return FileUploadRequest{Method: http.MethodPut, URL: contentURL + "?partNumber=" + strconv.Itoa(int(input.PartNumber)),
			Headers: map[string]string{"Authorization": "Bearer " + meta.Token}}, nil
	}
	setting, err := o.getS3Setting.ExecuteForOrganization(ctx, record.OrganizationID)
	if err != nil {
		return FileUploadRequest{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	if record.MultipartUploadID == nil {
		return FileUploadRequest{}, o.fileOperationError(ctx, meta, fileaction.ErrFileNotFound, cervii18n.ErrorFileUploadCreateFailed)
	}
	request, err := serverfilecontent.PresignPart(ctx, s3FileConfig(setting), record.StorageKey, *record.MultipartUploadID, input.PartNumber, size)
	if err != nil {
		return FileUploadRequest{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed)
	}
	return FileUploadRequest{Method: request.Method, URL: request.URL, Headers: request.Headers}, nil
}

// finalizeFileContent 合并尚未完成的分片并核验最终对象。
func (o *directOperations) finalizeFileContent(ctx context.Context, record *servermodels.File) (string, int64, error) {
	if record.PartSize > 0 {
		if record.StorageBackend == string(domain.FileStorageBackendLocal) {
			if err := o.localFiles.CompleteMultipart(ctx, record.StorageKey, record.ByteSize, record.PartSize); err != nil {
				return "", 0, err
			}
		} else {
			setting, err := o.getS3Setting.ExecuteForOrganization(ctx, record.OrganizationID)
			if err != nil {
				return "", 0, err
			}
			if record.MultipartUploadID == nil {
				return "", 0, fileaction.ErrFileNotFound
			}
			err = serverfilecontent.CompleteMultipart(ctx, s3FileConfig(setting), record.StorageKey, *record.MultipartUploadID, record.ByteSize, record.PartSize)
			var missing *types.NoSuchUpload
			// 合并响应丢失后，最终对象用于确认上一次合并结果。
			if err != nil && !errors.As(err, &missing) {
				return "", 0, fmt.Errorf("complete multipart upload: %w", err)
			}
		}
	}
	return o.statFile(ctx, record)
}

// CancelFileUpload 将当前用户取消的临时文件交给过期清理。
func (o *directOperations) CancelFileUpload(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, fileID string) error {
	if _, err := o.getFile.Execute(ctx, identity, fileID); err != nil {
		return o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed)
	}
	if err := o.cancelFileUpload.Execute(ctx, identity, fileID); err != nil {
		return o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed)
	}
	return nil
}
