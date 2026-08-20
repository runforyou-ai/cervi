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

// TestInstallRejectsOrganizationNameLongerThanLimit 验证初始化操作拒绝过长的公司名称。
func TestInstallRejectsOrganizationNameLongerThanLimit(t *testing.T) {
	action := NewInstallWorkspaceAction(nil)
	_, err := action.Execute(context.Background(), InstallWorkspaceInput{
		OrganizationName: strings.Repeat("名", maxOrganizationNameLength+1),
		DisplayName:      "所有者",
		Email:            "owner@example.com",
		Password:         "password123",
	})
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["organizationName"] != ValidationOrganizationNameTooLong {
		t.Fatalf("error = %#v, want organization name too long", err)
	}
}
