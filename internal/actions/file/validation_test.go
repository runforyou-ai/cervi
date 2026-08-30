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
	if key := storageKey("org", "file", normalized.ContentType); key != "organizations/org/files/file.png" {
		t.Fatalf("storage key = %q", key)
	}

	_, fields = normalizeUploadInput(UploadInput{Purpose: domain.FilePurposeUserAvatar, FileName: "avatar.svg", ContentType: "image/svg+xml", ByteSize: maxAvatarByteSize + 1})
	if fields["contentType"] != ValidationContentTypeInvalid || fields["byteSize"] != ValidationByteSizeInvalid {
		t.Fatalf("invalid fields = %#v", fields)
	}
	_, fields = normalizeUploadInput(UploadInput{Purpose: domain.FilePurposeContactAvatar, FileName: "avatar.jpg", ContentType: "image/jpeg", ByteSize: 3})
	if fields["purpose"] != ValidationPurposeInvalid {
		t.Fatalf("client contact avatar fields = %#v", fields)
	}
	_, fields = normalizeFileInput(UploadInput{Purpose: domain.FilePurposeContactAvatar, FileName: "avatar.jpg", ContentType: "image/jpeg", ByteSize: 3}, domain.FilePurposeContactAvatar)
	if len(fields) != 0 {
		t.Fatalf("imported contact avatar fields = %#v", fields)
	}
}
