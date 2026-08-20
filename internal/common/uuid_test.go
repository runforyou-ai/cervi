package common

import "testing"

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
