//go:build server

package conversation

import (
	"fmt"
	"strings"
	"testing"
)

// TestNormalizeGroupConversationInput 验证群聊名称和初始成员编号归一化。
func TestNormalizeGroupConversationInput(t *testing.T) {
	currentIdentityID := "0198ddee-c056-7bc5-a1d9-586f878ee966"
	memberIdentityID := "0198DDF0-A234-7F01-8D99-E3E0AF0F5F65"
	normalized, fields := normalizeGroupConversationInput(currentIdentityID, GroupConversationInput{
		Title:             "  产品讨论  ",
		MemberIdentityIDs: []string{memberIdentityID},
	})
	if len(fields) != 0 || normalized.Title != "产品讨论" || normalized.MemberIdentityIDs[0] != strings.ToLower(memberIdentityID) {
		t.Fatalf("normalized = %#v, fields = %#v", normalized, fields)
	}
}

// TestNormalizeGroupConversationInputRejectsInvalidMembers 验证群聊拒绝自己、重复成员和空成员集合。
func TestNormalizeGroupConversationInputRejectsInvalidMembers(t *testing.T) {
	currentIdentityID := "0198ddee-c056-7bc5-a1d9-586f878ee966"
	memberIdentityID := "0198ddf0-a234-7f01-8d99-e3e0af0f5f65"
	tests := []GroupConversationInput{
		{Title: "产品讨论"},
		{Title: "产品讨论", MemberIdentityIDs: []string{currentIdentityID}},
		{Title: "产品讨论", MemberIdentityIDs: []string{memberIdentityID, memberIdentityID}},
	}
	for _, input := range tests {
		_, fields := normalizeGroupConversationInput(currentIdentityID, input)
		if _, invalid := fields["memberIdentityIds"]; !invalid {
			t.Fatalf("input = %#v, fields = %#v", input, fields)
		}
	}
}

// TestNormalizeGroupConversationInputRejectsOversizedGroup 验证群聊初始参与者总数不超过一百人。
func TestNormalizeGroupConversationInputRejectsOversizedGroup(t *testing.T) {
	members := make([]string, 0, maxGroupParticipantCount)
	for index := 0; index < maxGroupParticipantCount; index++ {
		members = append(members, fmt.Sprintf("0198ddf0-a234-7f01-8d99-%012x", index))
	}
	_, fields := normalizeGroupConversationInput("0198ddee-c056-7bc5-a1d9-586f878ee966", GroupConversationInput{
		Title:             "大型群聊",
		MemberIdentityIDs: members,
	})
	if fields["memberIdentityIds"] != ValidationGroupMembersTooMany {
		t.Fatalf("validation fields = %#v", fields)
	}
}
