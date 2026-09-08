//go:build server

package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type isolatedChatRuntime struct{ expected map[string][]string }

// Run 校验每个 Run 只接收所属会话的历史。
func (r *isolatedChatRuntime) Run(ctx context.Context, request agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
	triggers, err := feed.Peek(ctx, 0)
	if err != nil {
		return agentruntime.RunResult{}, err
	}
	claimed, err := feed.Claim(ctx, triggers[len(triggers)-1].Seq)
	if err != nil {
		return agentruntime.RunResult{}, err
	}
	expected := r.expected[request.RunID]
	if len(claimed.Messages) != len(expected) {
		return agentruntime.RunResult{}, fmt.Errorf("context length=%d expected=%d", len(claimed.Messages), len(expected))
	}
	for i, message := range claimed.Messages {
		if message.Content != expected[i] {
			return agentruntime.RunResult{}, fmt.Errorf("context leaked: %q expected=%q", message.Content, expected[i])
		}
	}
	return agentruntime.RunResult{Content: "答复：" + expected[0], EndSeq: claimed.EndSeq}, nil
}

type failingChatScheduler struct{ inner *agentrunaction.Scheduler }

// Schedule 在真实输入和任务创建后返回失败以验证整个首发事务回滚。
func (s failingChatScheduler) Schedule(ctx context.Context, db bun.IDB, organizationID, conversationID, agentID, revisionID, messageID string) error {
	if err := s.inner.Schedule(ctx, db, organizationID, conversationID, agentID, revisionID, messageID); err != nil {
		return err
	}
	return errors.New("test scheduling failure")
}

// testAgentConversations 验证独立 AI 会话的创建幂等、上下文及访问边界。
func testAgentConversations(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string) {
	t.Helper()
	ctx := context.Background()
	agent, err := agentaction.NewCreateAgentAction(db).Execute(ctx, identity, agentaction.CreateInput{
		DisplayName: "独立会话助手", RoleID: roleID,
		Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{ProviderID: providerID, ModelIdentifier: modelID, SystemInstruction: "按当前会话回答"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tasks := servertask.New(db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	scheduler := agentrunaction.NewScheduler(tasks)
	start := conversationaction.NewSendFirstAgentTextMessageAction(db, scheduler)
	firstInput := conversationaction.FirstAgentTextMessageInput{ConversationID: uuid.NewV7().String(), AgentIdentityID: agent.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "任务甲"}
	if found, err := db.NewSelect().Model((*servermodels.Conversation)(nil)).Where("id = ?", firstInput.ConversationID).Exists(ctx); err != nil || found {
		t.Fatalf("draft persisted: %v %v", found, err)
	}
	// 同一首发并发重试只确认同一条消息。
	results := make([]conversationaction.FirstAgentTextMessageResult, 4)
	failures := make([]error, 4)
	var wg sync.WaitGroup
	for i := range results {
		wg.Go(func() { results[i], failures[i] = start.Execute(ctx, identity, firstInput) })
	}
	wg.Wait()
	for i, result := range results {
		if failures[i] != nil || result.Conversation.ID != firstInput.ConversationID || result.Message.ID != results[0].Message.ID {
			t.Fatalf("retry %d: %+v %v", i, result, failures[i])
		}
	}
	first := results[0]
	secondInput := firstInput
	secondInput.ConversationID, secondInput.ClientMessageID, secondInput.Body = uuid.NewV7().String(), uuid.NewV7().String(), "任务乙"
	second, err := start.Execute(ctx, identity, secondInput)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first.Conversation.ID, second.Conversation.ID} {
		for _, table := range []string{"messages", "conversation_agent_triggers", "agent_runs", "agent_conversations"} {
			count, err := db.NewSelect().TableExpr(table).Where("conversation_id = ?", id).Count(ctx)
			if err != nil || count != 1 {
				t.Fatalf("%s rows=%d err=%v", table, count, err)
			}
		}
	}
	altered := firstInput
	altered.Body = "被改动的重试"
	if _, err := start.Execute(ctx, identity, altered); err == nil {
		t.Fatal("changed idempotent body accepted")
	}
	reused := secondInput
	reused.ConversationID = uuid.NewV7().String()
	if _, err := start.Execute(ctx, identity, reused); err == nil {
		t.Fatal("message identifier reused across conversations")
	}
	// 执行两个会话，模型输入和输出分别归属自己的 Conversation。
	runs := make([]servermodels.AgentRun, 0)
	if err := db.NewSelect().Model(&runs).Where("agr.conversation_id IN (?, ?)", first.Conversation.ID, second.Conversation.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	runtime := &isolatedChatRuntime{expected: map[string][]string{}}
	for _, run := range runs {
		body := "任务甲"
		if run.ConversationID == second.Conversation.ID {
			body = "任务乙"
		}
		runtime.expected[run.ID] = []string{body}
	}
	executor := agentrunaction.NewExecuteAction(db, tasks, runtime)
	for _, run := range runs {
		if err := executor.Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
			t.Fatal(err)
		}
		var saved servermodels.AgentRun
		if err := db.NewSelect().Model(&saved).Where("agr.id = ?", run.ID).Scan(ctx); err != nil || saved.Status != string(domain.AgentRunStatusSucceeded) {
			t.Fatalf("run failed: %+v %v", saved, err)
		}
	}
	// Agent 已回复后重放首发，确认消息不变，摘要保留当前水位和运行状态。
	replay, err := start.Execute(ctx, identity, firstInput)
	if err != nil {
		t.Fatal(err)
	}
	summary := replay.Conversation
	if replay.Message.ID != first.Message.ID || summary.LastMessageID == nil || *summary.LastMessageID == first.Message.ID || summary.LastReadMessageID == nil || *summary.LastReadMessageID != first.Message.ID || summary.UnreadCount != 1 || summary.Agent.AgentRunStatus == nil || *summary.Agent.AgentRunStatus != domain.AgentRunStatusSucceeded {
		t.Fatalf("replayed summary = %+v, agent = %+v", summary, summary.Agent)
	}
	testAgentConversationAccess(t, db, identity, scheduler, first, second)
	rollbackInput := firstInput
	rollbackInput.ConversationID, rollbackInput.ClientMessageID = uuid.NewV7().String(), uuid.NewV7().String()
	if _, err := conversationaction.NewSendFirstAgentTextMessageAction(db, failingChatScheduler{scheduler}).Execute(ctx, identity, rollbackInput); err == nil {
		t.Fatal("scheduler failure ignored")
	}
	for _, table := range []string{"agent_conversations", "conversation_participants", "messages", "conversation_agent_states", "conversation_agent_triggers", "agent_runs"} {
		count, err := db.NewSelect().TableExpr(table).Where("conversation_id = ?", rollbackInput.ConversationID).Count(ctx)
		if err != nil || count != 0 {
			t.Fatalf("rollback %s rows=%d err=%v", table, count, err)
		}
	}
	if exists, err := db.NewSelect().Model((*servermodels.Conversation)(nil)).Where("id = ?", rollbackInput.ConversationID).Exists(ctx); err != nil || exists {
		t.Fatalf("empty conversation survived rollback: %v %v", exists, err)
	}
	t.Run("AI 会话事务锁序", func(t *testing.T) {
		testAgentChatLocking(t, db, identity, agent.ID, agent.IdentityID, tasks)
	})
}

// testAgentConversationAccess 验证多会话列表、阅读状态、引用和参与者访问范围。
func testAgentConversationAccess(t *testing.T, db *bun.DB, identity *servermodels.Identity, scheduler *agentrunaction.Scheduler, first, second conversationaction.FirstAgentTextMessageResult) {
	t.Helper()
	ctx := context.Background()
	send := conversationaction.NewSendAgentTextMessageAction(db, scheduler)
	if _, err := send.Execute(ctx, identity, conversationaction.InternalTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "跨会话引用", ReplyToMessageID: second.Message.ID}); err == nil {
		t.Fatal("cross conversation reference accepted")
	}
	history := conversationaction.NewListConversationMessagesQuery(db)
	for _, result := range []conversationaction.FirstAgentTextMessageResult{first, second} {
		page, err := history.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: result.Conversation.ID})
		if err != nil || len(page.Messages) != 2 || page.Messages[1].Body != "答复："+result.Message.Body {
			t.Fatalf("history: %+v %v", page, err)
		}
	}
	mark := conversationaction.NewMarkConversationReadAction(db)
	page, _ := history.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: first.Conversation.ID})
	if _, err := mark.Execute(ctx, identity, first.Conversation.ID, page.Messages[1].ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewUpdateConversationNotificationSettingsAction(db).Execute(ctx, identity, second.Conversation.ID, true); err != nil {
		t.Fatal(err)
	}
	unreadMark := conversationaction.NewUpdateConversationUnreadMarkAction(db)
	if err := unreadMark.Execute(ctx, identity, first.Conversation.ID, true); err != nil {
		t.Fatal(err)
	}
	rows, _, err := inboxaction.NewLoadInboxQuery(db).Execute(ctx, identity, inboxaction.LoadInput{Scope: domain.InboxScopeInternal})
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, row := range rows {
		if row.ID != first.Conversation.ID && row.ID != second.Conversation.ID {
			continue
		}
		found++
		if row.Agent == nil || row.Direct != nil || row.Agent.AgentIdentityID != first.Conversation.Agent.AgentIdentityID {
			t.Fatalf("AI inbox payload: %+v", row)
		}
		if row.ID == first.Conversation.ID && (row.UnreadCount != 0 || row.Muted || !row.MarkedUnread) {
			t.Fatalf("first read state: %+v", row)
		}
		if row.ID == second.Conversation.ID && (row.UnreadCount != 1 || !row.Muted || row.MarkedUnread) {
			t.Fatalf("second read state: %+v", row)
		}
	}
	if found != 2 {
		t.Fatalf("AI inbox rows=%d", found)
	}
	if err := unreadMark.Execute(ctx, identity, first.Conversation.ID, false); err != nil {
		t.Fatal(err)
	}
	outsider := newNavigationFixture(t)
	for _, actor := range []*servermodels.Identity{outsider.owner, outsider.member} {
		if _, err := history.Execute(ctx, actor, conversationaction.ConversationMessageHistoryInput{ConversationID: first.Conversation.ID}); !errors.Is(err, conversationaction.ErrConversationNotFound) {
			t.Fatalf("outside history: %v", err)
		}
		if _, err := send.Execute(ctx, actor, conversationaction.InternalTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "无权发送"}); !errors.Is(err, conversationaction.ErrConversationNotFound) {
			t.Fatalf("outside send: %v", err)
		}
	}
	if _, err := conversationaction.NewSendFirstDirectTextMessageAction(db).Execute(ctx, identity, conversationaction.FirstDirectTextMessageInput{TargetIdentityID: first.Conversation.Agent.AgentIdentityID, ClientMessageID: uuid.NewV7().String(), Body: "旧入口"}); !errors.Is(err, conversationaction.ErrDirectTargetNotFound) {
		t.Fatalf("direct accepted Agent: %v", err)
	}
	if _, err := send.Execute(ctx, identity, conversationaction.InternalTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "继续任务甲"}); err != nil {
		t.Fatal(err)
	}
}
