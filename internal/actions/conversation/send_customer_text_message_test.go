//go:build server

package conversation

import (
	"errors"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestNormalizeCustomerTextMessageInput 验证成员消息输入按 Unicode 字符校验。
func TestNormalizeCustomerTextMessageInput(t *testing.T) {
	input := CustomerTextMessageInput{
		ConversationID:  "0198DDEE-C056-7BC5-A1D9-586F878EE966",
		ClientMessageID: "0198DDF0-A234-7F01-8D99-E3E0AF0F5F65",
		Body:            "  回复客户  ",
	}
	normalized, fields := normalizeCustomerTextMessageInput(input)
	if len(fields) != 0 || normalized.Body != "回复客户" || normalized.ConversationID != strings.ToLower(input.ConversationID) || normalized.ClientMessageID != strings.ToLower(input.ClientMessageID) {
		t.Fatalf("normalized = %#v, fields = %#v", normalized, fields)
	}

	input.Body = strings.Repeat("鹿", 4001)
	_, fields = normalizeCustomerTextMessageInput(input)
	if fields["body"] != ValidationBodyTooLong {
		t.Fatalf("fields = %#v", fields)
	}
}

// TestPlanMemberReplySession 验证成员回复的领取规则。
func TestPlanMemberReplySession(t *testing.T) {
	currentIdentityID := "identity-current"
	otherIdentityID := "identity-other"
	tests := []struct {
		name       string
		status     domain.ServiceSessionStatus
		assignee   *string
		wantAssign bool
		wantReason string
		wantData   bool
	}{
		{name: "open public queue", status: domain.ServiceSessionStatusOpen, wantAssign: true},
		{name: "open assigned to current", status: domain.ServiceSessionStatusOpen, assignee: &currentIdentityID},
		{name: "assigned to other", status: domain.ServiceSessionStatusOpen, assignee: &otherIdentityID, wantReason: ConflictReasonServiceSessionOwned},
		{name: "closed", status: domain.ServiceSessionStatusClosed, assignee: &currentIdentityID, wantReason: ConflictReasonServiceSessionNotReplyable},
		{name: "unknown", status: domain.ServiceSessionStatus("unknown"), wantData: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := planMemberReplySession(test.status, test.assignee, currentIdentityID)
			if test.wantData {
				if !errors.Is(err, ErrDataInvariant) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if test.wantReason != "" {
				var conflict *ConflictError
				if !errors.As(err, &conflict) || conflict.Reason != test.wantReason {
					t.Fatalf("error = %#v", err)
				}
				return
			}
			if err != nil || plan.assign != test.wantAssign {
				t.Fatalf("plan = %#v, error = %v", plan, err)
			}
		})
	}
}

// TestMemberMessageMatches 验证幂等消息绑定原会话、正文和发送者。
func TestMemberMessageMatches(t *testing.T) {
	conversationID := "0198ddee-c056-7bc5-a1d9-586f878ee966"
	sessionID := "0198ddf0-a234-7f01-8d99-e3e0af0f5f65"
	participantID := "0198ddf0-a234-7f01-8d99-e3e0af0f5f66"
	subjectID := "0198ddf0-a234-7f01-8d99-e3e0af0f5f67"
	subjectKind := string(domain.ChatSubjectKindOrganizationIdentity)
	identityID := "0198ddf0-a234-7f01-8d99-e3e0af0f5f68"
	row := idempotentMemberMessageRow{
		ConversationID: conversationID, ServiceSessionID: &sessionID, JoinedServiceSessionID: &sessionID,
		SenderParticipantID: &participantID, SenderSubjectID: &subjectID,
		SenderSubjectKind: &subjectKind, SenderSubjectSourceID: &identityID,
		Type: string(domain.MessageTypeText), Body: "回复客户",
	}
	input := CustomerTextMessageInput{ConversationID: conversationID, Body: "回复客户"}
	if !memberMessageMatches(row, identityID, input.ConversationID, input.Body, true) {
		t.Fatal("expected idempotent message to match")
	}
	input.Body = "另一条回复"
	if memberMessageMatches(row, identityID, input.ConversationID, input.Body, true) {
		t.Fatal("expected changed body to conflict")
	}
	input.Body = row.Body
	input.ConversationID = "0198ddee-c056-7bc5-a1d9-586f878ee977"
	if memberMessageMatches(row, identityID, input.ConversationID, input.Body, true) {
		t.Fatal("expected changed conversation to conflict")
	}
	input.ConversationID = conversationID
	if memberMessageMatches(row, "0198ddf0-a234-7f01-8d99-e3e0af0f5f69", input.ConversationID, input.Body, true) {
		t.Fatal("expected changed sender to conflict")
	}
}
