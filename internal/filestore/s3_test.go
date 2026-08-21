//go:build server

package filestore

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestPresignRequests 验证对象存储上传和读取请求无需访问远端即可签发。
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

	get, err := PresignGet(context.Background(), config, "organizations/org/files/file.png", "image/png", "头像.png")
	if err != nil {
		t.Fatal(err)
	}
	if get.Method != http.MethodGet || !strings.Contains(get.URL, "X-Amz-Signature=") {
		t.Fatalf("get request = %#v", get)
	}
}
