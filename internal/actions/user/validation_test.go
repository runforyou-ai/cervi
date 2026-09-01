//go:build server

package user

import "testing"

// TestNormalizeProfileInput 验证个人资料输入规范化和字段校验。
func TestNormalizeProfileInput(t *testing.T) {
	normalized, fields := normalizeProfileInput(ProfileInput{
		DisplayName:  "  林晓  ",
		Email:        " LIN@Example.com ",
		AvatarFileID: " file-1 ",
	})
	if len(fields) != 0 {
		t.Fatalf("fields = %#v, want empty", fields)
	}
	if normalized.DisplayName != "林晓" || normalized.Email != "lin@example.com" || normalized.AvatarFileID != "file-1" {
		t.Fatalf("normalized input = %#v", normalized)
	}

	_, fields = normalizeProfileInput(ProfileInput{Email: "invalid"})
	if fields["displayName"] != ValidationDisplayNameRequired || fields["email"] != ValidationEmailInvalid {
		t.Fatalf("fields = %#v", fields)
	}
}

// TestNormalizeCreateInput 验证新增成员字段会规范化并校验角色与密码。
func TestNormalizeCreateInput(t *testing.T) {
	input, fields := normalizeCreateInput(CreateInput{DisplayName: "  林晓  ", Email: " LIN@EXAMPLE.COM ", Password: "password123", RoleID: "0198ddee-c056-7bc5-a1d9-586f878ee966"})
	if len(fields) != 0 || input.DisplayName != "林晓" || input.Email != "lin@example.com" {
		t.Fatalf("input = %#v, fields = %#v", input, fields)
	}
	_, fields = normalizeCreateInput(CreateInput{DisplayName: "", Email: "bad", Password: "short", RoleID: "invalid"})
	if fields["displayName"] != ValidationDisplayNameRequired || fields["email"] != ValidationEmailInvalid || fields["password"] != ValidationPasswordTooShort || fields["roleId"] != ValidationRoleInvalid {
		t.Fatalf("fields = %#v", fields)
	}
}
