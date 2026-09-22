package domain

import (
	"strings"
	"testing"
)

// TestDomainPrefixValid 验证企业域名前缀的 DNS 标签规则。
func TestDomainPrefixValid(t *testing.T) {
	for prefix, want := range map[string]bool{
		"acme":                  true,
		"a":                     true,
		"acme-01":               true,
		"0acme":                 true,
		strings.Repeat("a", 63): true,
		strings.Repeat("a", 64): false,
		"":                      false,
		"-acme":                 false,
		"acme-":                 false,
		"ac_me":                 false,
		"ac.me":                 false,
		"Acme":                  false,
		"中文":                    false,
	} {
		if got := DomainPrefixValid(prefix); got != want {
			t.Fatalf("DomainPrefixValid(%q) = %v", prefix, got)
		}
	}
	if got := NormalizeDomainPrefix("  AcMe "); got != "acme" {
		t.Fatalf("NormalizeDomainPrefix = %q", got)
	}
}
