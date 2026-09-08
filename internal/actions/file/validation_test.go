//go:build server

package file

import (
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestNormalizeUploadInput 验证图片上传元数据规范化和限制。
func TestNormalizeUploadInput(t *testing.T) {
	normalized, fields := NormalizeUploadInput(UploadInput{
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
	_, fields = NormalizeUploadInput(UploadInput{Purpose: domain.FilePurposeGroupImage, FileName: "group.webp", ContentType: "image/webp", ByteSize: 2048})
	if len(fields) != 0 {
		t.Fatalf("group image fields = %#v, want empty", fields)
	}

	_, fields = NormalizeUploadInput(UploadInput{Purpose: domain.FilePurposeUserAvatar, FileName: "avatar.svg", ContentType: "image/svg+xml", ByteSize: maxImageByteSize + 1})
	if fields["contentType"] != ValidationContentTypeInvalid || fields["byteSize"] != ValidationByteSizeInvalid {
		t.Fatalf("invalid fields = %#v", fields)
	}
	_, fields = NormalizeUploadInput(UploadInput{Purpose: domain.FilePurposeContactAvatar, FileName: "avatar.jpg", ContentType: "image/jpeg", ByteSize: 3})
	if fields["purpose"] != ValidationPurposeInvalid {
		t.Fatalf("client contact avatar fields = %#v", fields)
	}
	_, fields = normalizeFileInput(UploadInput{Purpose: domain.FilePurposeContactAvatar, FileName: "avatar.jpg", ContentType: "image/jpeg", ByteSize: 3}, domain.FilePurposeContactAvatar)
	if len(fields) != 0 {
		t.Fatalf("imported contact avatar fields = %#v", fields)
	}
}

// TestKnowledgeDocumentUploadFormats 验证允许格式、内容类型归一化和大文件不受头像上限限制。
func TestKnowledgeDocumentUploadFormats(t *testing.T) {
	for _, name := range []string{"a.txt", "a.md", "a.markdown", "a.htm", "a.html", "a.PDF", "a.docx", "a.pptx", "a.xlsx", "a.csv", "a.json"} {
		input, fields := NormalizeUploadInput(UploadInput{Purpose: domain.FilePurposeKnowledgeDocument, FileName: name, ContentType: "application/octet-stream", ByteSize: 1 << 40})
		if len(fields) != 0 || input.ContentType == "" || input.ContentType == "application/octet-stream" {
			t.Fatalf("%s: %+v %+v", name, input, fields)
		}
	}
	for _, name := range []string{"a.doc", "a.xls", "a.ppt", "a.exe", "a.pdf.exe", "a"} {
		_, fields := NormalizeUploadInput(UploadInput{Purpose: domain.FilePurposeKnowledgeDocument, FileName: name, ByteSize: 12})
		if fields["contentType"] != ValidationContentTypeInvalid {
			t.Fatalf("accepted %s", name)
		}
	}
}
