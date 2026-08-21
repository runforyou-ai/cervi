//go:build server

package filecontent

import "testing"

// TestContentFileID 验证文件内容服务能解析 Wails 移除挂载路径后的请求。
func TestContentFileID(t *testing.T) {
	fileID, ok := contentFileID("file-1/content")
	if !ok || fileID != "file-1" {
		t.Fatalf("contentFileID() = %q, %v", fileID, ok)
	}
	if _, ok := contentFileID("/files/file-1"); ok {
		t.Fatal("expected incomplete content path to fail")
	}
}

// TestContentDisposition 验证可预览类型内联展示，其他类型下载。
func TestContentDisposition(t *testing.T) {
	if value := contentDisposition("image/png", "头像.png"); value[:6] != "inline" {
		t.Fatalf("image disposition = %q", value)
	}
	if value := contentDisposition("application/vnd.openxmlformats-officedocument.wordprocessingml.document", "方案.docx"); value[:10] != "attachment" {
		t.Fatalf("document disposition = %q", value)
	}
}
