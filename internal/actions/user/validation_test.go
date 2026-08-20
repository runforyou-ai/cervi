//go:build server

package user

import (
	"strings"
	"testing"
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
