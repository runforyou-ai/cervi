//go:build server

package filecontent

import "testing"

// TestPublicURL 验证公开基础地址会保留路径并拒绝越界对象键。
func TestPublicURL(t *testing.T) {
	key := "organizations/org/files/file.png"
	value, err := PublicURL("https://cdn.example.com/assets/", key)
	if err != nil || value != "https://cdn.example.com/assets/organizations/org/files/file.png" {
		t.Fatalf("PublicURL() = %q, %v", value, err)
	}
	local, err := PublicURL("/storage", key)
	if err != nil || local != "/storage/organizations/org/files/file.png" {
		t.Fatalf("local PublicURL() = %q, %v", local, err)
	}
	for _, invalid := range []string{"../file.png", "/file.png", `organizations\\file.png`} {
		if _, err := PublicURL("https://cdn.example.com", invalid); err == nil {
			t.Fatalf("expected key %q to fail", invalid)
		}
	}
}
