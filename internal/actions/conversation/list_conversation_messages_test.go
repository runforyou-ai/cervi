//go:build server

package conversation

import (
	"fmt"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestValidateConversationMessageHistoryInputRejectsMixedCursors 验证成员消息查询不接受两个方向。
func TestValidateConversationMessageHistoryInputRejectsMixedCursors(t *testing.T) {
	point := MessageCursorPoint{OriginatedAt: time.Unix(1, 0), ID: "0198ddf0-a234-7f01-8d99-e3e0af0f5f65"}
	fields := validateConversationMessageHistoryInput(ConversationMessageHistoryInput{
		ConversationID: "0198ddee-c056-7bc5-a1d9-586f878ee966",
		Before:         &point,
		After:          &point,
	})
	if fields["cursor"] != ValidationCursorInvalid {
		t.Fatalf("validation fields = %#v", fields)
	}
}

// TestBuildConversationMessageHistoryReturnsInitialPageInAscendingOrder 验证初始消息页正序和双向边界。
func TestBuildConversationMessageHistoryReturnsInitialPageInAscendingOrder(t *testing.T) {
	rows := makeConversationMessageRows(conversationMessagePageSize + 1)
	wantFirstID := rows[conversationMessagePageSize-1].ID
	wantLastID := rows[0].ID
	history := buildConversationMessageHistory(rows, ConversationMessageHistoryInput{})
	if len(history.Messages) != conversationMessagePageSize {
		t.Fatalf("message count = %d", len(history.Messages))
	}
	if history.Messages[0].ID != wantFirstID || history.Messages[conversationMessagePageSize-1].ID != wantLastID {
		t.Fatalf("message order = %s ... %s", history.Messages[0].ID, history.Messages[conversationMessagePageSize-1].ID)
	}
	if history.Before == nil || history.Before.ID != history.Messages[0].ID {
		t.Fatalf("before = %#v", history.Before)
	}
	if history.After == nil || history.After.ID != history.Messages[conversationMessagePageSize-1].ID {
		t.Fatalf("after = %#v", history.After)
	}
	if history.Messages[0].Sender == nil || history.Messages[0].Sender.Kind != domain.ChatSubjectKindContact {
		t.Fatalf("sender = %#v", history.Messages[0].Sender)
	}
}

// TestBuildConversationMessageHistoryReturnsAfterPageInAscendingOrder 验证增量消息页保持查询正序。
func TestBuildConversationMessageHistoryReturnsAfterPageInAscendingOrder(t *testing.T) {
	rows := makeConversationMessageRows(2)
	history := buildConversationMessageHistory(rows, ConversationMessageHistoryInput{After: &MessageCursorPoint{}})
	if history.Messages[0].ID != rows[0].ID || history.Messages[1].ID != rows[1].ID {
		t.Fatalf("message order = %s, %s", history.Messages[0].ID, history.Messages[1].ID)
	}
	if history.Before != nil || history.After == nil || history.After.ID != rows[1].ID {
		t.Fatalf("cursors = before %#v, after %#v", history.Before, history.After)
	}
}

// TestBuildConversationMessageHistoryPreservesMissingSender 验证无发送主体的消息仍保留在时间线中。
func TestBuildConversationMessageHistoryPreservesMissingSender(t *testing.T) {
	rows := makeConversationMessageRows(1)
	rows[0].SenderSubjectID = nil
	rows[0].SenderKind = nil
	rows[0].SenderDisplayName = nil
	history := buildConversationMessageHistory(rows, ConversationMessageHistoryInput{})
	if len(history.Messages) != 1 || history.Messages[0].Sender != nil {
		t.Fatalf("messages = %#v", history.Messages)
	}
}

// TestBuildConversationMessageHistoryMarksSessionOpeningMessage 验证单个处理周期也保留开始标记。
func TestBuildConversationMessageHistoryMarksSessionOpeningMessage(t *testing.T) {
	rows := makeConversationMessageRows(3)
	openingMessageID := rows[2].ID
	sequence := int64(1)
	startedAt := time.Unix(1, 0)
	status := string(domain.ServiceSessionStatusActive)
	for index := range rows {
		rows[index].ServiceSessionOpeningMessageID = &openingMessageID
		rows[index].ServiceSessionSequence = &sequence
		rows[index].ServiceSessionStartedAt = &startedAt
		rows[index].ServiceSessionStatus = &status
	}

	history := buildConversationMessageHistory(rows, ConversationMessageHistoryInput{})
	if len(history.Messages) != 3 || history.Messages[0].SessionStart == nil {
		t.Fatalf("messages = %#v", history.Messages)
	}
	if history.Messages[0].SessionStart.Sequence != sequence || history.Messages[0].SessionStart.StartedAt != startedAt || history.Messages[0].SessionStart.Status != domain.ServiceSessionStatusActive {
		t.Fatalf("session start = %#v", history.Messages[0].SessionStart)
	}
	if history.Messages[1].SessionStart != nil || history.Messages[2].SessionStart != nil {
		t.Fatalf("non-opening session starts = %#v, %#v", history.Messages[1].SessionStart, history.Messages[2].SessionStart)
	}
}

// TestBuildConversationMessageHistoryMarksEachSessionOpeningMessage 验证多个处理周期分别标记首条消息。
func TestBuildConversationMessageHistoryMarksEachSessionOpeningMessage(t *testing.T) {
	rows := makeConversationMessageRows(4)
	firstOpeningMessageID := rows[3].ID
	secondOpeningMessageID := rows[1].ID
	firstSequence, secondSequence := int64(1), int64(2)
	firstStartedAt, secondStartedAt := time.Unix(1, 0), time.Unix(3, 0)
	closedStatus := string(domain.ServiceSessionStatusClosed)
	activeStatus := string(domain.ServiceSessionStatusActive)
	for index := 2; index < 4; index++ {
		rows[index].ServiceSessionOpeningMessageID = &firstOpeningMessageID
		rows[index].ServiceSessionSequence = &firstSequence
		rows[index].ServiceSessionStartedAt = &firstStartedAt
		rows[index].ServiceSessionStatus = &closedStatus
	}
	for index := 0; index < 2; index++ {
		rows[index].ServiceSessionOpeningMessageID = &secondOpeningMessageID
		rows[index].ServiceSessionSequence = &secondSequence
		rows[index].ServiceSessionStartedAt = &secondStartedAt
		rows[index].ServiceSessionStatus = &activeStatus
	}

	history := buildConversationMessageHistory(rows, ConversationMessageHistoryInput{})
	if history.Messages[0].SessionStart == nil || history.Messages[0].SessionStart.Sequence != firstSequence {
		t.Fatalf("first session start = %#v", history.Messages[0].SessionStart)
	}
	if history.Messages[2].SessionStart == nil || history.Messages[2].SessionStart.Sequence != secondSequence {
		t.Fatalf("second session start = %#v", history.Messages[2].SessionStart)
	}
	if history.Messages[1].SessionStart != nil || history.Messages[3].SessionStart != nil {
		t.Fatalf("non-opening session starts = %#v, %#v", history.Messages[1].SessionStart, history.Messages[3].SessionStart)
	}
}

// makeConversationMessageRows 创建查询已经按目标方向排列的消息行。
func makeConversationMessageRows(count int) []conversationMessageRow {
	displayName := "访客"
	subjectID := "0198ddf0-a234-7f01-8d99-e3e0af0f0000"
	subjectKind := string(domain.ChatSubjectKindContact)
	rows := make([]conversationMessageRow, 0, count)
	for index := 0; index < count; index++ {
		rows = append(rows, conversationMessageRow{
			ID:   fmt.Sprintf("0198ddf0-a234-7f01-8d99-e3e0af0f%04x", index),
			Type: string(domain.MessageTypeText), Body: fmt.Sprintf("消息 %d", index),
			OriginatedAt: time.Unix(int64(count-index), 0), CreatedAt: time.Unix(int64(count-index), 0),
			SenderSubjectID: &subjectID,
			SenderKind:      &subjectKind, SenderDisplayName: &displayName,
		})
	}
	return rows
}
