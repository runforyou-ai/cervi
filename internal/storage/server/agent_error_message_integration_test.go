//go:build server

package server

import (
	"context"
	"errors"
	"testing"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// testAgentFailureMessages 验证失败结果作为普通时间线记录持久化并排除于模型上下文。
func testAgentFailureMessages(t *testing.T, db *bun.DB, identity *servermodels.Identity, tasks *servertask.Runtime, conversationID, firstRunID, secondRunID, nextRunID string) {
	t.Helper()
	ctx := context.Background()
	query := conversationaction.NewListConversationMessagesQuery(db)
	failures := make(map[string]bool)
	finalizer := agentrunaction.NewExecuteAction(db, tasks, nil)
	for _, runID := range []string{firstRunID, secondRunID} {
		var run servermodels.AgentRun
		if err := db.NewSelect().Model(&run).Where("agr.id = ?", runID).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if run.ResponseMessageID == nil {
			t.Fatal("failed run has no result message")
		}
		var message servermodels.Message
		if err := db.NewSelect().Model(&message).Where("msg.id = ?", *run.ResponseMessageID).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if message.Type != string(domain.MessageTypeAgentError) || message.Body != "" || message.SenderParticipantID == nil || message.IdempotencyKey == nil || *message.IdempotencyKey != "agent:"+runID {
			t.Fatalf("failure message = %+v", message)
		}
		failures[message.ID] = true
		if err := finalizer.FinalizeFailure(ctx, agentrunaction.RunInput{RunID: runID}, errors.New("duplicate completion")); err != nil {
			t.Fatal(err)
		}
	}
	history, err := query.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, message := range history.Messages {
		if !failures[message.ID] {
			continue
		}
		count++
		if message.AgentProcess != nil || message.Sender == nil || message.Sender.IdentityType == nil || *message.Sender.IdentityType != domain.OrganizationIdentityTypeAgent {
			t.Fatalf("failure sender = %+v", message)
		}
		cursor := &conversationaction.MessageCursorPoint{ID: message.ID, MessageSeq: message.MessageSeq}
		earlier, err := query.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID, Before: cursor})
		if err != nil || earlier.After == nil {
			t.Fatalf("error message pagination = %+v, %v", earlier, err)
		}
		later, err := query.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID, After: earlier.After})
		if err != nil || len(later.Messages) == 0 || later.Messages[0].ID != message.ID {
			t.Fatalf("error message incremental page = %+v, %v", later, err)
		}
	}
	if count != 2 {
		t.Fatalf("failure message count = %d", count)
	}
	runtime := testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		pending, err := feed.Peek(ctx, 0)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		claimed, err := feed.Claim(ctx, pending[len(pending)-1].Seq)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		for _, message := range claimed.Messages {
			if failures[message.ID] {
				t.Fatal("failure entered model context")
			}
		}
		return agentruntime.RunResult{EndSeq: claimed.EndSeq, Content: "恢复后的回复"}, nil
	}}
	if err := agentrunaction.NewExecuteAction(db, tasks, runtime).Execute(ctx, agentrunaction.RunInput{RunID: nextRunID}); err != nil {
		t.Fatal(err)
	}
	restored, err := query.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID})
	if err != nil {
		t.Fatal(err)
	}
	count = 0
	for _, message := range restored.Messages {
		if message.Type == domain.MessageTypeAgentError {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("next reply replaced error messages: %d", count)
	}
}

// testCustomerFailureMessage 验证客服失败消息进入成员历史，访客摘要和历史仅展示正常消息。
func testCustomerFailureMessage(t *testing.T, db *bun.DB, identity *servermodels.Identity, tasks *servertask.Runtime, agentID string) {
	t.Helper()
	ctx := context.Background()
	channel, err := channelaction.NewCreateMessageChannelAction(db).Execute(ctx, identity, channelaction.CreateMessageChannelInput{
		Type: domain.ChannelTypeWebsite, Name: "首响失败验证", DefaultLocale: domain.LocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeMember, ID: agentID}, FallbackTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
	})
	if err != nil {
		t.Fatal(err)
	}
	receive := conversationaction.NewReceiveWebsiteCustomerTextMessageAction(db, agentrunaction.NewScheduler(tasks))
	input := conversationaction.WebsiteCustomerTextMessageInput{ChannelID: channel.ID, ExternalID: "web-session:0123456789abcdef0123456789abcdef", ClientMessageID: uuid.NewV7().String(), Body: "首条客服消息"}

	sent, err := receive.Execute(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	var run servermodels.AgentRun
	if err := db.NewSelect().Model(&run).Where("agr.conversation_id = ? AND agr.status = ?", sent.Conversation.ID, domain.AgentRunStatusQueued).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if err := agentrunaction.NewExecuteAction(db, tasks, nil).FinalizeFailure(ctx, agentrunaction.RunInput{RunID: run.ID}, errors.New("model failure details")); err != nil {
		t.Fatal(err)
	}
	var session servermodels.ServiceSession
	if err := db.NewSelect().Model(&session).Where("ss.id = ?", *run.ServiceSessionID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if session.FirstResponseAt != nil {
		t.Fatal("failure counted as first customer response")
	}

	history, err := conversationaction.NewListConversationMessagesQuery(db).Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: sent.Conversation.ID})
	if err != nil || len(history.Messages) == 0 || history.Messages[len(history.Messages)-1].Type != domain.MessageTypeAgentError {
		t.Fatalf("member error history = %+v, %v", history, err)
	}
	inboxPage, _, err := inboxaction.NewLoadInboxQuery(db).Execute(ctx, identity, inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewCoworkers, AssigneeIdentityID: run.AgentIdentityID})
	inbox := inboxPage.Conversations
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range inbox {
		if row.ID == sent.Conversation.ID {
			found = true
			if row.LastMessageType == nil || *row.LastMessageType != domain.MessageTypeAgentError || row.UnreadCount != 2 {
				t.Fatalf("error preview = %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("customer conversation missing from inbox")
	}
	visible, err := conversationaction.NewListWebsiteConversationsQuery(db).Execute(ctx, input.ChannelID, input.ExternalID)
	if err != nil || len(visible) != 1 || visible[0].Preview != input.Body {
		t.Fatalf("visitor preview = %+v, %v", visible, err)
	}
	replay, err := receive.Execute(ctx, input)
	if err != nil || replay.Conversation.Preview != input.Body {
		t.Fatalf("visitor replay preview = %+v, %v", replay, err)
	}
	messages, err := conversationaction.NewListWebsiteMessagesQuery(db).Execute(ctx, conversationaction.MessageHistoryInput{ChannelID: input.ChannelID, ExternalID: input.ExternalID, ConversationID: sent.Conversation.ID})
	if err != nil || len(messages.Messages) == 0 || messages.Messages[len(messages.Messages)-1].ID != sent.Message.ID {
		t.Fatalf("visitor history = %+v, %v", messages, err)
	}
}
