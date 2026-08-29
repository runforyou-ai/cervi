package connectiontest

import (
	"net/url"
	"testing"
)

// TestAppendPathPreservesEscapedSegments 验证外部资源编号不会越过路径段边界。
func TestAppendPathPreservesEscapedSegments(t *testing.T) {
	requestURL, err := AppendPath(
		"https://example.com/custom/v1",
		"datasets/"+url.PathEscape("dataset/1")+"/documents/"+url.PathEscape("document/1"),
	)
	if err != nil {
		t.Fatalf("append path: %v", err)
	}
	parsed, err := url.Parse(requestURL)
	if err != nil {
		t.Fatalf("parse request URL: %v", err)
	}
	if parsed.EscapedPath() != "/custom/v1/datasets/dataset%2F1/documents/document%2F1" {
		t.Fatalf("escaped path = %q", parsed.EscapedPath())
	}
}
