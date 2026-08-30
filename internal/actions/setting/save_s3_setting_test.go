//go:build server

package setting

import (
	"context"
	"errors"
	"testing"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestSaveS3SettingRejectsInvalidIdentity 验证保存前会拒绝无效的用户企业关联。
func TestSaveS3SettingRejectsInvalidIdentity(t *testing.T) {
	input := S3Setting{
		Provider:        domain.StorageProviderAWS,
		Endpoint:        "https://s3.us-east-1.amazonaws.com",
		PublicBaseURL:   "https://cdn.example.com",
		Region:          "us-east-1",
		Bucket:          "example",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
	}
	tests := []struct {
		name     string
		identity *servermodels.Identity
	}{
		{name: "nil identity"},
		{
			name: "invalid identifiers",
			identity: &servermodels.Identity{
				Organization: servermodels.Organization{ID: "invalid"},
				User:         servermodels.User{ID: "invalid", OrganizationID: "invalid"},
			},
		},
		{
			name: "mismatched organization",
			identity: &servermodels.Identity{
				Organization: servermodels.Organization{ID: "fe41c981-49a9-44f7-996c-c29bc7fd6600"},
				User: servermodels.User{
					ID:             "be2b59bb-9067-47f8-b085-8b173898799c",
					OrganizationID: "6f28a3cd-2dcd-4ce7-94b1-965e06d71ae0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSaveS3SettingAction(nil).Execute(context.Background(), tt.identity, input)
			if !errors.Is(err, common.ErrIdentityInvalid) {
				t.Fatalf("Execute() error = %v, want common.ErrIdentityInvalid", err)
			}
		})
	}
}
