//go:build server

package installation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestInstallRejectsPasswordLongerThanBcryptLimit 验证初始化操作拒绝超过 bcrypt 上限的密码。
func TestInstallRejectsPasswordLongerThanBcryptLimit(t *testing.T) {
	action := NewInstallWorkspaceAction(nil)
	_, err := action.Execute(context.Background(), InstallWorkspaceInput{
		OrganizationName: "鹿行测试公司",
		DisplayName:      "所有者",
		Email:            "owner@example.com",
		Password:         strings.Repeat("中", 25),
	})
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["password"] == "" {
		t.Fatalf("error = %#v, want password validation error", err)
	}
}
