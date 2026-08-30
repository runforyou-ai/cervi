//go:build server

package filecontent

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestPresignRequests 验证对象存储上传请求携带不可变缓存元数据。
func TestPresignRequests(t *testing.T) {
	config := S3Config{
		Endpoint: "https://storage.example.com", Region: "us-east-1", Bucket: "cervi",
		AccessKeyID: "access-key", SecretAccessKey: "secret-key", ForcePathStyle: true,
	}
	put, err := PresignPut(context.Background(), config, "organizations/org/files/file.png", "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if put.Method != http.MethodPut || !strings.HasPrefix(put.URL, "https://storage.example.com/cervi/") {
		t.Fatalf("put request = %#v", put)
	}
	if _, exists := put.Headers["Host"]; exists {
		t.Fatalf("browser upload request contains forbidden Host header: %#v", put.Headers)
	}
	if put.Headers["Cache-Control"] != ImmutableCacheControl {
		t.Fatalf("cache control = %q, want %q", put.Headers["Cache-Control"], ImmutableCacheControl)
	}
	if put.Headers["Content-Type"] != "image/png" {
		t.Fatalf("content type = %q, want image/png", put.Headers["Content-Type"])
	}
}
