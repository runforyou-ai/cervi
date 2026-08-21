//go:build server

// Package file 实现企业文件领域的应用操作。
package file

import (
	"mime"
	"path/filepath"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const maxAvatarByteSize int64 = 5 * 1024 * 1024

// ValidationCode 标识文件字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationFileNameRequired   ValidationCode = "FILE_NAME_REQUIRED"
	ValidationContentTypeInvalid ValidationCode = "CONTENT_TYPE_INVALID"
	ValidationByteSizeInvalid    ValidationCode = "BYTE_SIZE_INVALID"
	ValidationPurposeInvalid     ValidationCode = "PURPOSE_INVALID"
)

// ValidationError 表示文件字段校验失败。
type ValidationError = common.FieldError

var avatarContentTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// originalObjectName 返回带规范化后缀的原始文件对象名。
func originalObjectName(input UploadInput) string {
	return "original" + avatarContentTypes[input.ContentType]
}

// UploadInput 定义待上传文件的客户端元数据。
type UploadInput struct {
	Purpose     domain.FilePurpose
	FileName    string
	ContentType string
	ByteSize    int64
}

// normalizeUploadInput 规范化并校验待上传文件元数据。
func normalizeUploadInput(input UploadInput) (UploadInput, map[string]ValidationCode) {
	input.FileName = strings.TrimSpace(filepath.Base(strings.ReplaceAll(input.FileName, "\\", "/")))
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(input.ContentType))
	if err == nil {
		input.ContentType = strings.ToLower(mediaType)
	} else {
		input.ContentType = ""
	}
	fields := make(map[string]ValidationCode)
	if input.FileName == "" || input.FileName == "." || len([]rune(input.FileName)) > 255 {
		fields["fileName"] = ValidationFileNameRequired
	}
	if input.Purpose != domain.FilePurposeUserAvatar {
		fields["purpose"] = ValidationPurposeInvalid
	}
	if _, exists := avatarContentTypes[input.ContentType]; !exists {
		fields["contentType"] = ValidationContentTypeInvalid
	}
	if input.ByteSize <= 0 || input.ByteSize > maxAvatarByteSize {
		fields["byteSize"] = ValidationByteSizeInvalid
	}
	return input, fields
}
