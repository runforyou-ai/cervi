//go:build server

package setting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// TestS3SettingActionExecute 验证 S3 连接测试会签名并访问目标存储桶。
func TestS3SettingActionExecute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodHead {
			t.Errorf("method = %q, want HEAD", request.Method)
		}
		if request.URL.Path != "/cervi" {
			t.Errorf("path = %q, want /cervi", request.URL.Path)
		}
		if request.Header.Get("Authorization") == "" {
			t.Error("missing signed authorization header")
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	action := NewTestS3SettingAction(connectiontest.NewRunner(time.Second))
	err := action.Execute(context.Background(), S3Setting{
		Enabled:         true,
		Provider:        domain.StorageProviderGeneric,
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "cervi",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestS3SettingActionReturnsConnectionError 验证存储桶拒绝访问时返回稳定错误。
func TestS3SettingActionReturnsConnectionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	action := NewTestS3SettingAction(connectiontest.NewRunner(time.Second))
	err := action.Execute(context.Background(), S3Setting{
		Enabled:         true,
		Provider:        domain.StorageProviderGeneric,
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "cervi",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		ForcePathStyle:  true,
	})
	stage, kind, ok := connectiontest.Details(err)
	if !ok || stage != connectiontest.StageAuthorize || kind != connectiontest.FailureForbidden {
		t.Fatalf("stage = %q, kind = %q, ok = %v", stage, kind, ok)
	}
}
