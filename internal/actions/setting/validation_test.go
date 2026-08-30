//go:build server

package setting

import (
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestNormalizeS3Setting 验证 S3 配置规范化和字段校验。
func TestNormalizeS3Setting(t *testing.T) {
	normalized, fields := normalizeS3Setting(S3Setting{
		Enabled:         true,
		Provider:        domain.StorageProviderAWS,
		Endpoint:        " https://s3.example.com ",
		PublicBaseURL:   " https://cdn.example.com/assets/ ",
		Region:          " us-east-1 ",
		Bucket:          " cervi ",
		AccessKeyID:     " access-key ",
		SecretAccessKey: " secret-key ",
	})
	if len(fields) != 0 {
		t.Fatalf("unexpected validation fields: %#v", fields)
	}
	if normalized.Endpoint != "https://s3.example.com" || normalized.PublicBaseURL != "https://cdn.example.com/assets" || normalized.Bucket != "cervi" {
		t.Fatalf("unexpected normalized setting: %#v", normalized)
	}

	normalized.Provider = domain.StorageProviderMinIO
	if _, minioFields := normalizeS3Setting(normalized); len(minioFields) != 0 {
		t.Fatalf("unexpected MinIO validation fields: %#v", minioFields)
	}

	_, fields = normalizeS3Setting(S3Setting{Provider: "unknown", Endpoint: "s3.example.com?query=value", PublicBaseURL: "cdn.example.com?query=value"})
	for _, name := range []string{"provider", "endpoint", "publicBaseUrl", "region", "bucket", "accessKeyId", "secretAccessKey"} {
		if _, exists := fields[name]; !exists {
			t.Fatalf("missing validation field %q in %#v", name, fields)
		}
	}
}
