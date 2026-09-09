//go:build server

package filecontent

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// Reader 按原件记录中的存储类型流式读取文件。
type Reader struct {
	local     *LocalStore
	resolveS3 S3ConfigResolver
}

// NewReader 创建后台原件读取器。
func NewReader(local *LocalStore, resolveS3 S3ConfigResolver) *Reader {
	return &Reader{local: local, resolveS3: resolveS3}
}

// Open 打开本地原件或对象存储响应体，由调用方关闭。
func (r *Reader) Open(ctx context.Context, file *servermodels.File) (io.ReadCloser, error) {
	switch domain.FileStorageBackend(file.StorageBackend) {
	case domain.FileStorageBackendLocal:
		content, _, err := r.local.Open(ctx, file.StorageKey)
		return content, err
	case domain.FileStorageBackendS3:
		config, err := r.resolveS3(ctx, file.OrganizationID)
		if err != nil {
			return nil, err
		}
		output, err := newS3Client(config).GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(config.Bucket), Key: aws.String(file.StorageKey)})
		if err != nil {
			return nil, err
		}
		return output.Body, nil
	default:
		return nil, fmt.Errorf("invalid file storage backend %q", file.StorageBackend)
	}
}
