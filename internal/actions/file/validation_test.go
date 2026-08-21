//go:build server

package file

import (
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestNormalizeUploadInput 验证头像上传元数据规范化和限制。
func TestNormalizeUploadInput(t *testing.T) {
	normalized, fields := normalizeUploadInput(UploadInput{
		Purpose: domain.FilePurposeUserAvatar, FileName: `C:\fakepath\avatar.png`, ContentType: "image/png; charset=binary", ByteSize: 1024,
	})
	if len(fields) != 0 {
		t.Fatalf("fields = %#v, want empty", fields)
	}
	if normalized.FileName != "avatar.png" || normalized.ContentType != "image/png" {
		t.Fatalf("normalized input = %#v", normalized)
	}
	if originalObjectName(normalized) != "original.png" {
		t.Fatalf("object name = %q, want original.png", originalObjectName(normalized))
	}

	_, fields = normalizeUploadInput(UploadInput{Purpose: domain.FilePurposeUserAvatar, FileName: "avatar.svg", ContentType: "image/svg+xml", ByteSize: maxAvatarByteSize + 1})
	if fields["contentType"] != ValidationContentTypeInvalid || fields["byteSize"] != ValidationByteSizeInvalid {
		t.Fatalf("invalid fields = %#v", fields)
	}
}
