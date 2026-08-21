//go:build server

package team

import (
	"strings"
	"testing"
)

// TestNormalizeInput 验证团队字段会被规范化并校验长度。
func TestNormalizeInput(t *testing.T) {
	input, fields := normalizeInput(Input{Name: "  客户成功  ", Description: "  服务客户  "})
	if len(fields) != 0 || input.Name != "客户成功" || input.Description != "服务客户" {
		t.Fatalf("input = %#v, fields = %#v", input, fields)
	}
	_, fields = normalizeInput(Input{Name: "", Description: strings.Repeat("介", 501)})
	if fields["name"] != ValidationNameRequired || fields["description"] != ValidationDescriptionTooLong {
		t.Fatalf("fields = %#v", fields)
	}
}

// TestNormalizePage 验证团队分页限制。
func TestNormalizePage(t *testing.T) {
	page, pageSize, valid := normalizePage(0, 0)
	if page != 1 || pageSize != 50 || !valid {
		t.Fatalf("page = %d, pageSize = %d, valid = %v", page, pageSize, valid)
	}
	_, _, valid = normalizePage(1, 101)
	if valid {
		t.Fatal("page size above 100 should be invalid")
	}
}
