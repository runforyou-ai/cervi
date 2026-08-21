//go:build server

package organization

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestUpdateOrganizationValidatesInputBeforeStorage 验证企业名称归一化和校验不依赖存储。
func TestUpdateOrganizationValidatesInputBeforeStorage(t *testing.T) {
	action := NewUpdateOrganizationAction(nil)
	for _, tt := range []struct {
		name  string
		input Input
		code  ValidationCode
	}{
		{name: "empty", input: Input{Name: "  "}, code: ValidationNameRequired},
		{name: "too long", input: Input{Name: strings.Repeat("鹿", domain.OrganizationNameMaxLength+1)}, code: ValidationNameTooLong},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := action.Execute(context.Background(), nil, tt.input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Fields["name"] != tt.code {
				t.Fatalf("Execute() error = %v, want %s", err, tt.code)
			}
		})
	}
}

// TestUpdateOrganizationRejectsInvalidIdentity 验证企业名称修改拒绝无效身份。
func TestUpdateOrganizationRejectsInvalidIdentity(t *testing.T) {
	identity := &servermodels.Identity{
		Organization: servermodels.Organization{ID: "fe41c981-49a9-44f7-996c-c29bc7fd6600"},
		User: servermodels.User{
			ID:             "be2b59bb-9067-47f8-b085-8b173898799c",
			OrganizationID: "6f28a3cd-2dcd-4ce7-94b1-965e06d71ae0",
		},
	}
	if _, err := NewUpdateOrganizationAction(nil).Execute(context.Background(), identity, Input{Name: "鹿行"}); !errors.Is(err, common.ErrIdentityInvalid) {
		t.Fatalf("Update Execute() error = %v, want common.ErrIdentityInvalid", err)
	}
}
