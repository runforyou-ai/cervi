package sessiontoken

import (
	"testing"
	"time"
)

// TestIssue 验证会话令牌签发结果。
func TestIssue(t *testing.T) {
	issued, err := Issue()
	if err != nil {
		t.Fatal(err)
	}
	if issued.Token == "" || issued.TokenHash != Hash(issued.Token) {
		t.Fatalf("unexpected issued token: %#v", issued)
	}
	remaining := time.Until(issued.ExpiresAt)
	if remaining < defaultDuration-time.Minute || remaining > defaultDuration {
		t.Fatalf("session duration = %v, want about %v", remaining, defaultDuration)
	}
}
