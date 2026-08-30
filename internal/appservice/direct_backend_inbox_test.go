//go:build server

package appservice

import "testing"

// TestInboxAvatarContentURL 验证联系人头像复用企业文件内容地址。
func TestInboxAvatarContentURL(t *testing.T) {
	fileID := "019d4e1c-40a5-77dd-82e6-6951f9957ba5"
	value := avatarContentURL(&fileID)
	if value != "/files/"+fileID+"/content" {
		t.Fatalf("avatar URL = %q", value)
	}
	if value := avatarContentURL(nil); value != "" {
		t.Fatalf("missing avatar URL = %q", value)
	}
}
