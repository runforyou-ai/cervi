//go:build server

package integrationtest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// testCustomerCopilotThreads 验证 Copilot 线程的创建、多人提问、背景资料、停止回复、会话隔离与实时受众。
func testCustomerCopilotThreads(t *testing.T, db *bun.DB, identity *servermodels.Identity, providerID, modelID string) {
	ctx := context.Background()
	created, err := agentaction.NewCreateAgentAction(db).Execute(ctx, identity, agentaction.CreateInput{
		HandlesCustomers: true, DisplayName: "Copilot 助手",
		Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{ProviderID: providerID, ModelIdentifier: modelID, SystemInstruction: "你是售后专家"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tasks := newTestTasks(db)
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	scheduler := agentrunaction.NewScheduler(tasks)
	channel, err := channelaction.NewCreateMessageChannelAction(db).Execute(ctx, identity, channelaction.CreateMessageChannelInput{
		Type: domain.ChannelTypeWebsite, Name: "Copilot 线程", DefaultLocale: domain.LocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue}, FallbackTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
	})
	if err != nil {
		t.Fatal(err)
	}
	inbound, err := conversationaction.NewReceiveWebsiteCustomerMessageAction(db, scheduler, newTestTasks(db)).Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: channel.ID, ExternalID: "web-session:" + strings.ReplaceAll(uuid.NewV7().String(), "-", ""), ClientMessageID: uuid.NewV7().String(), Body: "包裹显示签收但没收到",
	})
	if err != nil {
		t.Fatal(err)
	}
	customerID := inbound.Conversation.ID
	if _, err := conversationaction.NewSendCustomerTextMessageAction(db, nil).Execute(ctx, identity, conversationaction.CustomerTextMessageInput{
		ConversationID: customerID, ClientMessageID: uuid.NewV7().String(), Body: "我来帮您核实物流",
	}); err != nil {
		t.Fatal(err)
	}
	customerMessageCount, err := db.NewSelect().Model((*servermodels.Message)(nil)).Where("conversation_id = ?", customerID).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	colleague := newChatLockUser(t, db, identity)
	outsider := newNavigationFixture(t).owner
	startThread := conversationaction.NewSendFirstCustomerCopilotMessageAction(db, scheduler)
	ask := conversationaction.NewSendCustomerCopilotTextMessageAction(db, scheduler)
	listThreads := conversationaction.NewListCustomerCopilotThreadsQuery(db)
	listMessages := conversationaction.NewListConversationMessagesQuery(db)

	threadID := uuid.NewV7().String()
	firstInput := conversationaction.FirstCustomerCopilotMessageInput{
		ThreadID: threadID, CustomerConversationID: customerID, AgentIdentityID: created.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "这个客户之前退过款吗",
	}
	first, err := startThread.Execute(ctx, identity, firstInput)
	if err != nil {
		t.Fatal(err)
	}
	if first.Thread.ID != threadID || first.Thread.Title != firstInput.Body || first.Thread.AgentIdentityID != created.IdentityID || first.Thread.CreatedByIdentityID != identity.OrganizationIdentity.ID {
		t.Fatalf("first thread = %+v", first.Thread)
	}
	replayed, err := startThread.Execute(ctx, identity, firstInput)
	if err != nil || replayed.Message.ID != first.Message.ID {
		t.Fatalf("replayed first message = %+v, error = %v", replayed.Message, err)
	}
	mismatched := firstInput
	mismatched.CustomerConversationID = uuid.NewV7().String()
	var conflict *conversationaction.ConflictError
	if _, err := startThread.Execute(ctx, identity, mismatched); !errors.As(err, &conflict) {
		t.Fatalf("mismatched thread replay error = %v", err)
	}
	if _, err := ask.Execute(ctx, colleague, conversationaction.InternalTextMessageInput{ConversationID: threadID, ClientMessageID: uuid.NewV7().String(), Body: "物流单号能查到吗"}); err != nil {
		t.Fatal(err)
	}
	var run servermodels.AgentRun
	if err := db.NewSelect().Model(&run).Where("agr.conversation_id = ?", threadID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var kinds []string
	if err := db.NewSelect().Table("agent_inputs").Column("kind").Where("lane_id = ?", run.LaneID).Order("input_seq").Scan(ctx, &kinds); err != nil {
		t.Fatal(err)
	}
	if run.Status != string(domain.AgentRunStatusQueued) || run.ScopeKind != string(domain.AgentExecutionScopeConversation) || run.ScopeID != threadID ||
		len(kinds) != 2 || kinds[0] != string(domain.AgentInputKindCopilot) || kinds[1] != string(domain.AgentInputKindCopilot) {
		t.Fatalf("copilot run = %+v, input kinds = %v", run, kinds)
	}

	threads, err := listThreads.Execute(ctx, colleague, customerID)
	if err != nil || len(threads) != 1 || threads[0].ID != threadID || threads[0].AgentStatus != domain.UserStatusActive {
		t.Fatalf("colleague threads = %+v, error = %v", threads, err)
	}
	inboxPage, _, err := inboxaction.NewLoadInboxQuery(db).Execute(ctx, colleague, inboxaction.LoadInput{Scope: domain.InboxScopeChat})
	if err != nil {
		t.Fatal(err)
	}
	assertInboxConversationPresence(t, inboxPage.Conversations, threadID, false)
	search, err := inboxaction.NewLoadInboxQuery(db).Search(ctx, identity, inboxaction.SearchInput{Text: "物流单号", Range: inboxaction.SearchRangeReadable})
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range search.Messages {
		if message.Conversation.ID == threadID {
			t.Fatalf("copilot thread message appeared in search: %+v", message)
		}
	}
	var stateCount int
	if stateCount, err = db.NewSelect().Table("conversation_user_states").Where("conversation_id = ?", threadID).Count(ctx); err != nil || stateCount != 0 {
		t.Fatalf("copilot personal states = %d, error = %v", stateCount, err)
	}

	runtime := testAgentRuntime{run: func(ctx context.Context, request agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		claimed, err := feed.Claim(ctx, 2)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		if !strings.HasPrefix(request.Assignment.Instruction, "你是企业「") || !strings.Contains(request.Assignment.Instruction, "\n\n你是售后专家") || !strings.Contains(request.Assignment.Instruction, "customer_conversation_background") ||
			!strings.Contains(request.Assignment.Instruction, "customer-reply") || !strings.Contains(request.Assignment.Instruction, "与客户最近消息相同的语言") {
			t.Errorf("copilot instruction = %q", request.Assignment.Instruction)
		}
		if len(claimed.Messages) != 3 || !strings.HasPrefix(claimed.Messages[0].ID, "copilot-background:"+customerID+":") {
			t.Errorf("copilot context = %+v", claimed.Messages)
			return agentruntime.RunResult{Content: "上下文不符", EndSeq: claimed.EndSeq}, nil
		}
		background := claimed.Messages[0].Content
		for _, expected := range []string{`"kind":"customer_conversation_background"`, "包裹显示签收但没收到", "我来帮您核实物流", `"kind":"customer"`, `"kind":"member"`, `"status":"open"`} {
			if !strings.Contains(background, expected) {
				t.Errorf("background does not contain %q: %s", expected, background)
			}
		}
		if !strings.Contains(claimed.Messages[1].Content, `"name":"`+identity.OrganizationIdentity.DisplayName+`"`) ||
			!strings.Contains(claimed.Messages[2].Content, `"name":"`+colleague.OrganizationIdentity.DisplayName+`"`) {
			t.Errorf("copilot questions = %q / %q", claimed.Messages[1].Content, claimed.Messages[2].Content)
		}
		return agentruntime.RunResult{Content: "建议先核实物流签收凭证", EndSeq: claimed.EndSeq}, nil
	}}
	executor := agentrunaction.NewExecuteAction(db, tasks, runtime, testAttachmentReader(db), nil)
	if err := executor.Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	history, err := listMessages.Execute(ctx, colleague, conversationaction.ConversationMessageHistoryInput{ConversationID: threadID})
	if err != nil || len(history.Messages) != 3 || history.Messages[2].Body != "建议先核实物流签收凭证" ||
		history.Messages[2].Sender == nil || history.Messages[2].Sender.SourceID != created.IdentityID {
		t.Fatalf("copilot history = %+v, error = %v", history, err)
	}
	if count, err := db.NewSelect().Model((*servermodels.Message)(nil)).Where("conversation_id = ?", customerID).Count(ctx); err != nil || count != customerMessageCount {
		t.Fatalf("customer messages after copilot = %d, want %d, error = %v", count, customerMessageCount, err)
	}

	// 跨企业身份不能读取线程列表、线程消息或继续提问。
	if _, err := listThreads.Execute(ctx, outsider, customerID); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("outsider list threads error = %v", err)
	}
	if _, err := listMessages.Execute(ctx, outsider, conversationaction.ConversationMessageHistoryInput{ConversationID: threadID}); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("outsider thread history error = %v", err)
	}
	if _, err := ask.Execute(ctx, outsider, conversationaction.InternalTextMessageInput{ConversationID: threadID, ClientMessageID: uuid.NewV7().String(), Body: "越权提问"}); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("outsider ask error = %v", err)
	}

	feed := startRealtimeFeed(t, identity.Organization.ID)
	if _, err := ask.Execute(ctx, colleague, conversationaction.InternalTextMessageInput{ConversationID: threadID, ClientMessageID: uuid.NewV7().String(), Body: "还有别的办法吗"}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, feed.customerInbox(threadID, loadConversationVersion(t, db, threadID)))
	var next servermodels.AgentRun
	if err := db.NewSelect().Model(&next).Where("agr.conversation_id = ? AND agr.status = ?", threadID, domain.AgentRunStatusQueued).Scan(ctx); err != nil || next.InputStartSeq != 3 {
		t.Fatalf("next copilot run = %+v, error = %v", next, err)
	}
	if _, err := executor.StopCustomerCopilotReply(ctx, outsider, threadID, next.ID); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("outsider stop error = %v", err)
	}
	if status, err := executor.StopCustomerCopilotReply(ctx, identity, threadID, next.ID); err != nil || status != domain.AgentRunStatusCancelled {
		t.Fatalf("stop copilot reply = %s, error = %v", status, err)
	}
	feed.expect(t, feed.customerInbox(threadID, loadConversationVersion(t, db, threadID)))
	if count, err := db.NewSelect().Model((*servermodels.Message)(nil)).Where("conversation_id = ? AND type = ?", threadID, domain.MessageTypeAgentCancelled).Count(ctx); err != nil || count != 1 {
		t.Fatalf("copilot stopped messages = %d, error = %v", count, err)
	}

	// 停用 AI 员工后线程保留只读，新提问返回 AI 员工不可用。
	if _, err := db.NewUpdate().Table("agents").Set("status = ?", domain.UserStatusInactive).Where("identity_id = ?", created.IdentityID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.NewUpdate().Table("agents").Set("status = ?", domain.UserStatusActive).Where("identity_id = ?", created.IdentityID).Exec(context.Background())
	})
	if _, err := ask.Execute(ctx, colleague, conversationaction.InternalTextMessageInput{ConversationID: threadID, ClientMessageID: uuid.NewV7().String(), Body: "停用后提问"}); !errors.Is(err, conversationaction.ErrAgentUnavailable) {
		t.Fatalf("inactive agent ask error = %v", err)
	}
	if _, err := startThread.Execute(ctx, colleague, conversationaction.FirstCustomerCopilotMessageInput{
		ThreadID: uuid.NewV7().String(), CustomerConversationID: customerID, AgentIdentityID: created.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "新对话",
	}); !errors.Is(err, conversationaction.ErrAgentUnavailable) {
		t.Fatalf("inactive agent new thread error = %v", err)
	}
	if threads, err = listThreads.Execute(ctx, identity, customerID); err != nil || len(threads) != 1 || threads[0].AgentStatus != domain.UserStatusInactive {
		t.Fatalf("threads after agent disabled = %+v, error = %v", threads, err)
	}
}
