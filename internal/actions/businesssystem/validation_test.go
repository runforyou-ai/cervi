//go:build server

package businesssystem

import (
	"strings"
	"testing"
)

// TestNormalizeInputAcceptsBusinessURLs 验证业务系统地址允许内网、端口、查询参数和片段。
func TestNormalizeInputAcceptsBusinessURLs(t *testing.T) {
	for _, address := range []string{
		"https://erp.example.com/workbench?tenant=cervi#orders",
		"http://192.168.1.20:8080/portal",
		"http://intranet.local/",
	} {
		input, fields := normalizeInput(Input{Name: " 企业 ERP ", URL: " " + address + " ", Enabled: true})
		if len(fields) != 0 {
			t.Fatalf("normalizeInput(%q) fields = %#v", address, fields)
		}
		if input.Name != "企业 ERP" || input.URL != address || !input.Enabled {
			t.Fatalf("normalizeInput(%q) = %#v", address, input)
		}
	}
}

// TestNormalizeInputRejectsInvalidBusinessURLs 验证业务系统地址必须完整且不能携带认证信息。
func TestNormalizeInputRejectsInvalidBusinessURLs(t *testing.T) {
	for _, address := range []string{
		"erp.example.com",
		"ftp://erp.example.com",
		"https://user:password@erp.example.com",
		"https:///portal",
	} {
		_, fields := normalizeInput(Input{Name: "企业 ERP", URL: address})
		if fields["url"] != ValidationURLInvalid {
			t.Fatalf("normalizeInput(%q) fields = %#v", address, fields)
		}
	}
	_, fields := normalizeInput(Input{Name: "企业 ERP", URL: "https://example.com/" + strings.Repeat("a", 2048)})
	if fields["url"] != ValidationURLTooLong {
		t.Fatalf("long URL fields = %#v", fields)
	}
}

// TestNormalizeInputValidatesBusinessSystemName 验证业务系统名称必填且长度受限。
func TestNormalizeInputValidatesBusinessSystemName(t *testing.T) {
	_, emptyFields := normalizeInput(Input{Name: " ", URL: "https://example.com"})
	if emptyFields["name"] != ValidationNameRequired {
		t.Fatalf("empty name fields = %#v", emptyFields)
	}
	_, longFields := normalizeInput(Input{Name: strings.Repeat("鹿", 101), URL: "https://example.com"})
	if longFields["name"] != ValidationNameTooLong {
		t.Fatalf("long name fields = %#v", longFields)
	}
}
