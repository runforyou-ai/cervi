//go:build server

package appservice

import "testing"

// TestInboxAvatarURL 验证联系人头像使用已解析的文件公开地址。
func TestInboxAvatarURL(t *testing.T) {
	fileID := "019d4e1c-40a5-77dd-82e6-6951f9957ba5"
	value := optionalFileURL(map[string]string{fileID: "https://cdn.example.com/avatar.png"}, &fileID)
	if value != "https://cdn.example.com/avatar.png" {
		t.Fatalf("avatar URL = %q", value)
	}
	if value := optionalFileURL(nil, nil); value != "" {
		t.Fatalf("missing avatar URL = %q", value)
	}
}
