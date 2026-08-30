package tenant

import "testing"

// TestNormalizeAccessHost 验证租户访问地址保留非默认端口。
func TestNormalizeAccessHost(t *testing.T) {
	tests := map[string]string{
		"":                    "",
		"EXAMPLE.COM.":        "example.com",
		"example.com:80":      "example.com",
		"example.com:443":     "example.com",
		"example.com:8443":    "example.com:8443",
		"localhost:8080":      "localhost:8080",
		"127.0.0.1":           "127.0.0.1",
		"127.0.0.1:80":        "127.0.0.1",
		"127.0.0.1:8080":      "127.0.0.1:8080",
		"[::1]:443":           "::1",
		"[::1]:8080":          "[::1]:8080",
		"[::1]":               "::1",
		"example.com:0":       "",
		"example.com:invalid": "",
		"example.com:65536":   "",
	}
	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			if actual := NormalizeAccessHost(input); actual != expected {
				t.Fatalf("NormalizeAccessHost(%q) = %q, want %q", input, actual, expected)
			}
		})
	}
}
