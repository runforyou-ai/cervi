package token

import (
	"testing"
	"time"
)

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
		t.Fatalf("token duration = %v, want about %v", remaining, defaultDuration)
	}
}
