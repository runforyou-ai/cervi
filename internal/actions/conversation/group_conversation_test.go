//go:build server

package conversation

import (
	"context"
	"errors"
	"fmt"
	"testing"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestCreateGroupConversationRejectsInvalidMembers 验证群聊创建拒绝无效的初始成员集合。
func TestCreateGroupConversationRejectsInvalidMembers(t *testing.T) {
	currentIdentityID := "0198ddee-c056-7bc5-a1d9-586f878ee966"
	memberIdentityID := "0198ddf0-a234-7f01-8d99-e3e0af0f5f65"
	oversizedMembers := make([]string, 0, 100)
	for index := 0; index < 100; index++ {
		oversizedMembers = append(oversizedMembers, fmt.Sprintf("0198ddf0-a234-7f01-8d99-%012x", index))
	}
	tests := []struct {
		name  string
		input GroupConversationInput
		code  ValidationCode
	}{
		{name: "空成员", input: GroupConversationInput{Title: "产品讨论"}, code: ValidationGroupMembersRequired},
		{name: "包含自己", input: GroupConversationInput{Title: "产品讨论", MemberIdentityIDs: []string{currentIdentityID}}, code: ValidationGroupMemberIDsInvalid},
		{name: "重复成员", input: GroupConversationInput{Title: "产品讨论", MemberIdentityIDs: []string{memberIdentityID, memberIdentityID}}, code: ValidationGroupMemberIDsInvalid},
		{name: "超过人数上限", input: GroupConversationInput{Title: "大型群聊", MemberIdentityIDs: oversizedMembers}, code: ValidationGroupMembersTooMany},
	}
	identity := &servermodels.Identity{OrganizationIdentity: servermodels.OrganizationIdentity{ID: currentIdentityID}}
	action := NewCreateGroupConversationAction(nil)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := action.Execute(context.Background(), identity, test.input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Fields["memberIdentityIds"] != test.code {
				t.Fatalf("error = %#v, want memberIdentityIds %q", err, test.code)
			}
		})
	}
}
