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

// TestNormalizeCreateInput 验证新增成员字段会规范化并校验角色与密码。
func TestNormalizeCreateInput(t *testing.T) {
	input, fields := normalizeCreateInput(CreateInput{DisplayName: "  林晓  ", Email: " LIN@EXAMPLE.COM ", Password: "password123", Role: domain.UserRoleMember})
	if len(fields) != 0 || input.DisplayName != "林晓" || input.Email != "lin@example.com" {
		t.Fatalf("input = %#v, fields = %#v", input, fields)
	}
	_, fields = normalizeCreateInput(CreateInput{DisplayName: "", Email: "bad", Password: "short", Role: "owner"})
	if fields["displayName"] != ValidationDisplayNameRequired || fields["email"] != ValidationEmailInvalid || fields["password"] != ValidationPasswordTooShort || fields["role"] != ValidationRoleInvalid {
		t.Fatalf("fields = %#v", fields)
	}
}

// TestPreferencesValidation 验证语言和 IANA 时区校验。
func TestPreferencesValidation(t *testing.T) {
	fields := validatePreferencesInput(PreferencesInput{
		Locale: domain.LocaleChineseSimplified, TimeZone: "Asia/Shanghai",
	})
	if len(fields) != 0 {
		t.Fatalf("valid preferences fields = %#v, want empty", fields)
	}
	fields = validatePreferencesInput(PreferencesInput{Locale: "fr-FR", TimeZone: "invalid"})
	if fields["locale"] != ValidationLocaleInvalid || fields["timeZone"] != ValidationTimeZoneInvalid {
		t.Fatalf("invalid preferences fields = %#v", fields)
	}
}

// TestWorkStatusValidation 验证工作状态白名单。
func TestWorkStatusValidation(t *testing.T) {
	fields := validateWorkStatusInput(WorkStatusInput{WorkStatus: domain.WorkStatusAway})
	if len(fields) != 0 {
		t.Fatalf("valid work status fields = %#v", fields)
	}
	for _, status := range []domain.WorkStatus{" away ", "offline"} {
		fields = validateWorkStatusInput(WorkStatusInput{WorkStatus: status})
		if fields["workStatus"] != ValidationWorkStatusInvalid {
			t.Fatalf("work status %q fields = %#v", status, fields)
		}
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
