//go:build server

package user

import (
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestNormalizeProfileInput 验证个人资料输入规范化和字段校验。
func TestNormalizeProfileInput(t *testing.T) {
	normalized, fields := normalizeProfileInput(ProfileInput{
		DisplayName: "  林晓  ",
		Email:       " LIN@Example.com ",
	})
	if len(fields) != 0 {
		t.Fatalf("fields = %#v, want empty", fields)
	}
	if normalized.DisplayName != "林晓" || normalized.Email != "lin@example.com" {
		t.Fatalf("normalized input = %#v", normalized)
	}

	_, fields = normalizeProfileInput(ProfileInput{Email: "invalid"})
	if fields["displayName"] != ValidationDisplayNameRequired || fields["email"] != ValidationEmailInvalid {
		t.Fatalf("fields = %#v", fields)
	}
}

// TestPreferencesValidation 验证语言和 IANA 时区校验。
func TestPreferencesValidation(t *testing.T) {
	_, fields := normalizePreferencesInput(PreferencesInput{
		Locale: domain.LocaleChineseSimplified, TimeZone: "Asia/Shanghai",
	})
	if len(fields) != 0 {
		t.Fatalf("valid preferences fields = %#v, want empty", fields)
	}
	_, fields = normalizePreferencesInput(PreferencesInput{Locale: "fr-FR", TimeZone: "invalid"})
	if fields["locale"] != ValidationLocaleInvalid || fields["timeZone"] != ValidationTimeZoneInvalid {
		t.Fatalf("invalid preferences fields = %#v", fields)
	}
}

// TestWorkStatusValidation 验证工作状态白名单和空白规范化。
func TestWorkStatusValidation(t *testing.T) {
	normalized, fields := normalizeWorkStatusInput(WorkStatusInput{WorkStatus: " away "})
	if len(fields) != 0 || normalized.WorkStatus != domain.WorkStatusAway {
		t.Fatalf("valid work status = %#v, fields = %#v", normalized, fields)
	}
	_, fields = normalizeWorkStatusInput(WorkStatusInput{WorkStatus: "offline"})
	if fields["workStatus"] != ValidationWorkStatusInvalid {
		t.Fatalf("invalid work status fields = %#v", fields)
	}
}

// TestValidateChangePasswordInput 验证新密码长度规则。
func TestValidateChangePasswordInput(t *testing.T) {
	if fields := validateChangePasswordInput(ChangePasswordInput{NewPassword: "password123"}); len(fields) != 0 {
		t.Fatalf("valid password fields = %#v, want empty", fields)
	}
	if fields := validateChangePasswordInput(ChangePasswordInput{NewPassword: "short"}); fields["newPassword"] != ValidationPasswordTooShort {
		t.Fatalf("short password fields = %#v", fields)
	}
	if fields := validateChangePasswordInput(ChangePasswordInput{NewPassword: strings.Repeat("中", 25)}); fields["newPassword"] != ValidationPasswordTooLong {
		t.Fatalf("long password fields = %#v", fields)
	}
}
