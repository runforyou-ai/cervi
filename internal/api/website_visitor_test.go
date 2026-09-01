//go:build server

package api

import (
	"testing"
)

// TestGenerateWebsiteVisitorToken 验证访客令牌使用固定长度的随机十六进制格式。
func TestGenerateWebsiteVisitorToken(t *testing.T) {
	token, err := generateWebsiteVisitorToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if !validWebsiteVisitorToken(token) {
		t.Fatalf("invalid token format: %q", token)
	}
}

// TestValidWebsiteVisitorTokenRejectsNonHex 验证访客令牌不接受十六进制之外的字符。
func TestValidWebsiteVisitorTokenRejectsNonHex(t *testing.T) {
	if validWebsiteVisitorToken("gggggggggggggggggggggggggggggggg") {
		t.Fatal("expected non-hex token to be rejected")
	}
}
