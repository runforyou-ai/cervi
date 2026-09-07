//go:build server

package server

import (
	"context"
	"errors"
	"testing"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// testAgentFailureHistory 验证新运行、历史分页和空轮询均不会覆盖已失败的轮次。
func testAgentFailureHistory(t *testing.T, db *bun.DB, identity *servermodels.Identity, conversationID, firstFailedID, secondFailedID, nextRunID string) {
	t.Helper()
	ctx := context.Background()
	query := conversationaction.NewListConversationMessagesQuery(db)
	history, err := query.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID})
	if err != nil {
		t.Fatal(err)
	}
	if history.LatestAgentRun == nil || history.LatestAgentRun.ID != nextRunID || history.LatestAgentRun.Status != domain.AgentRunStatusQueued || len(history.AgentFailures) != 2 {
		t.Fatalf("new run replaced failure history: %+v", history)
	}
	if history.AgentFailures[0].ID != firstFailedID || history.AgentFailures[1].ID != secondFailedID {
		t.Fatalf("failure order = %+v", history.AgentFailures)
	}
	var secondInput conversationaction.ConversationMessage
	for _, message := range history.Messages {
		if message.ID == history.AgentFailures[1].AfterMessageID {
			secondInput = message
		}
	}
	if secondInput.Body != "失败后继续" {
		t.Fatalf("failure anchor = %+v", secondInput)
	}
	cursor := &conversationaction.MessageCursorPoint{ID: secondInput.ID, OriginatedAt: secondInput.OriginatedAt, SourceOrder: secondInput.SourceOrder}
	earlier, err := query.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID, Before: cursor})
	if err != nil || len(earlier.AgentFailures) != 1 || earlier.AgentFailures[0].ID != firstFailedID {
		t.Fatalf("earlier failure history = %+v, error = %v", earlier.AgentFailures, err)
	}
	// 游标之后的失败即使不再是最近一次运行，也随增量页返回。
	later, err := query.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID, After: cursor})
	if err != nil || len(later.AgentFailures) != 1 || later.AgentFailures[0].ID != secondFailedID {
		t.Fatalf("incremental failure history = %+v, error = %v", later.AgentFailures, err)
	}
	// 把游标推进到最新消息，再结束当前运行，模拟没有新消息的失败通知。
	finalizer := agentrunaction.NewExecuteAction(db, nil, nil, nil)
	if err := finalizer.FinalizeFailure(ctx, agentrunaction.RunInput{RunID: nextRunID}, errors.New("fixture failure")); err != nil {
		t.Fatal(err)
	}
	empty, err := query.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID, After: history.After})
	if err != nil || len(empty.Messages) != 0 || len(empty.AgentFailures) != 1 || empty.AgentFailures[0].ID != nextRunID || empty.AgentFailures[0].AfterMessageID != history.After.ID {
		t.Fatalf("empty polling page = %+v, error = %v", empty, err)
	}
}
