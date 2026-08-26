//go:build server

package organization

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestUpdateOrganizationValidatesInputBeforeStorage 验证企业名称输入先于存储校验。
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
