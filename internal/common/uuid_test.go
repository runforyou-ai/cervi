package common

import (
	"slices"
	"testing"
)

// TestValidUUID 验证只接受规范 UUID。
func TestValidUUID(t *testing.T) {
	if !ValidUUID("123e4567-e89b-12d3-a456-426614174000") {
		t.Fatal("canonical UUID should be accepted")
	}
	if !ValidUUID("123E4567-E89B-12D3-A456-426614174000") {
		t.Fatal("case-insensitive canonical UUID should be accepted")
	}
	if ValidUUID("urn:uuid:123e4567-e89b-12d3-a456-426614174000") {
		t.Fatal("URN UUID should not be accepted as a database identifier")
	}
	if ValidUUID("") {
		t.Fatal("empty value should not be accepted")
	}
}

// TestNormalizeUUID 验证 UUID 会去除空白并转换为规范形式。
func TestNormalizeUUID(t *testing.T) {
	normalized, valid := NormalizeUUID(" 123E4567-E89B-12D3-A456-426614174000 ")
	if !valid || normalized != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("NormalizeUUID() = %q, %v", normalized, valid)
	}
	if _, valid = NormalizeUUID("invalid"); valid {
		t.Fatal("invalid UUID should not be accepted")
	}
}

// TestNormalizeUUIDs 验证 UUID 列表会保持顺序并按规范值去重。
func TestNormalizeUUIDs(t *testing.T) {
	values, valid := NormalizeUUIDs([]string{
		"123E4567-E89B-12D3-A456-426614174000",
		" 123e4567-e89b-12d3-a456-426614174000 ",
		"123e4567-e89b-12d3-a456-426614174001",
	})
	want := []string{"123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001"}
	if !valid || !slices.Equal(values, want) {
		t.Fatalf("NormalizeUUIDs() = %#v, %v", values, valid)
	}
	if _, valid = NormalizeUUIDs([]string{"invalid"}); valid {
		t.Fatal("list containing invalid UUID should not be accepted")
	}
}
