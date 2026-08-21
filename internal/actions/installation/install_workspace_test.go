//go:build server

package installation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
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

// TestInstallRejectsInvalidLocaleAndTimeZone 验证初始化拒绝无效的浏览器区域设置。
func TestInstallRejectsInvalidLocaleAndTimeZone(t *testing.T) {
	action := NewInstallWorkspaceAction(nil)
	_, err := action.Execute(context.Background(), InstallWorkspaceInput{
		OrganizationName: "鹿行测试公司",
		DisplayName:      "所有者",
		Email:            "owner@example.com",
		Password:         "password123",
		Locale:           domain.Locale("fr-FR"),
		TimeZone:         "invalid",
	})
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["locale"] != ValidationLocaleInvalid || validationError.Fields["timeZone"] != ValidationTimeZoneInvalid {
		t.Fatalf("error = %#v, want locale and time zone validation", err)
	}
}

// TestInstallRejectsOrganizationNameLongerThanLimit 验证初始化操作拒绝过长的企业名称。
func TestInstallRejectsOrganizationNameLongerThanLimit(t *testing.T) {
	action := NewInstallWorkspaceAction(nil)
	_, err := action.Execute(context.Background(), InstallWorkspaceInput{
		OrganizationName: strings.Repeat("名", domain.OrganizationNameMaxLength+1),
		DisplayName:      "所有者",
		Email:            "owner@example.com",
		Password:         "password123",
	})
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["organizationName"] != ValidationOrganizationNameTooLong {
		t.Fatalf("error = %#v, want organization name too long", err)
	}
}
