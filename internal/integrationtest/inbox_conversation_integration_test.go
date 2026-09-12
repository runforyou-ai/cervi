//go:build server

package integrationtest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/appservice"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

// TestInboxIndependentConversation 验证首页外深链、批量逐项结果及退群和解散的阅读边界。
func TestInboxIndependentConversation(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := tenant.WithAccessHost(context.Background(), f.owner.Organization.AccessHost)
	for index := 0; index < 79; index++ {
		if _, err := conversationaction.NewCreateGroupConversationAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationInput{Title: fmt.Sprintf("分页外群 %d", index), MemberIdentityIDs: []string{f.member.OrganizationIdentity.ID}}); err != nil {
			t.Fatal(err)
		}
	}
	login, err := authaction.NewLoginAction(f.db).Execute(ctx, authaction.LoginInput{OrganizationID: f.owner.Organization.ID, Email: "member@navigation.test", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	backend := appservice.NewDirectBackend(f.db, nil, serverstorage.NewTenantResolver(f.db), nil, nil, nil, nil)
	meta := appservice.RequestMeta{Token: login.Token}
	inbox, err := backend.LoadInbox(ctx, meta, appservice.LoadInboxInput{Scope: appservice.InboxScopeInternal})
	if err != nil || len(inbox.Conversations) != 50 || !inbox.HasMore || inbox.NextCursor == "" {
		t.Fatalf("inbox=%+v err=%v", inbox, err)
	}
	for _, item := range inbox.Conversations {
		if item.ID == f.groupID {
			t.Fatal("oldest group unexpectedly in first page")
		}
	}
	second, err := backend.LoadInbox(ctx, meta, appservice.LoadInboxInput{Scope: appservice.InboxScopeInternal, Cursor: inbox.NextCursor})
	if err != nil || len(second.Conversations) != 30 || second.HasMore || second.NextCursor != "" || second.Conversations[29].ID != f.groupID {
		t.Fatalf("second page=%+v err=%v", second, err)
	}

	summary, err := backend.GetInboxConversation(ctx, meta, f.groupID)
	if err != nil || summary.ID != f.groupID || summary.Group == nil {
		t.Fatalf("deep link=%+v err=%v", summary, err)
	}
	missing := uuid.NewV7().String()
	foreign := newNavigationFixture(t)
	request := appservice.ReadInboxConversationsInput{ConversationIDs: []string{f.groupID, missing, foreign.groupID, f.groupID}, Query: appservice.InboxQuery{Scope: appservice.InboxScopeCustomer}}
	batch, err := backend.ReadInboxConversations(ctx, meta, request)
	if err != nil || len(batch.Results) != 4 {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	for index, item := range batch.Results {
		if item.ID != request.ConversationIDs[index] {
			t.Fatal("input order changed")
		}
		if index == 0 || index == 3 {
			if item.Availability != appservice.InboxConversationOutsideQuery || item.Conversation == nil {
				t.Fatalf("readable outside filter=%+v", item)
			}
		} else if item.Availability != appservice.InboxConversationUnavailable || item.Conversation != nil {
			t.Fatalf("invisible entity leaked=%+v", item)
		}
	}
	// 核验不存在和跨企业会话返回相同错误。
	for _, id := range []string{missing, foreign.groupID} {
		_, err := backend.GetInboxConversation(ctx, meta, id)
		var apiError *appservice.Error
		if !errors.As(err, &apiError) || apiError.Reason != "conversation_unavailable" {
			t.Fatalf("unavailable error=%v", err)
		}
	}
	if _, err := conversationaction.NewRemoveGroupConversationMemberAction(f.db, newGroupAgentCoordinator(f.db)).Execute(ctx, f.owner, conversationaction.GroupConversationMemberInput{ConversationID: f.groupID, MemberIdentityID: f.member.OrganizationIdentity.ID}); err != nil {
		t.Fatal(err)
	}
	request.Query.Scope = appservice.InboxScopeInternal
	batch, err = backend.ReadInboxConversations(ctx, meta, request)
	if err != nil || batch.Results[0].Conversation != nil || batch.Results[0].Availability != appservice.InboxConversationUnavailable {
		t.Fatalf("removed=%+v err=%v", batch, err)
	}
	if _, err := conversationaction.NewAddGroupConversationMembersAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationMembersInput{ConversationID: f.groupID, MemberIdentityIDs: []string{f.member.OrganizationIdentity.ID}}); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewDissolveGroupConversationAction(f.db, newGroupAgentCoordinator(f.db)).Execute(ctx, f.owner, f.groupID); err != nil {
		t.Fatal(err)
	}
	summary, err = backend.GetInboxConversation(ctx, meta, f.groupID)
	if err != nil || summary.Group == nil || summary.Group.Status != appservice.ConversationStatusArchived {
		t.Fatalf("dissolved=%+v err=%v", summary, err)
	}
	batch, err = backend.ReadInboxConversations(ctx, meta, request)
	if err != nil || batch.Results[0].Availability != appservice.InboxConversationMatching {
		t.Fatalf("rejoined=%+v err=%v", batch, err)
	}
}

// TestInboxCustomerDetailSnapshot 验证客服转交和关闭不撤销阅读，同轮批量摘要与资格保持一致。
func TestInboxCustomerDetailSnapshot(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := conversationaction.NewClaimServiceSessionAction(f.db, nil).Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatal(err)
	}
	direct, err := conversationaction.NewSendFirstDirectTextMessageAction(f.db).Execute(ctx, f.owner, conversationaction.FirstDirectTextMessageInput{TargetIdentityID: f.member.OrganizationIdentity.ID, ClientMessageID: uuid.NewV7().String(), Body: "独立单聊摘要"})
	if err != nil {
		t.Fatal(err)
	}
	query := inboxaction.NewLoadInboxQuery(f.db)
	mine := inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewMine}
	ids := []string{f.conversationID, direct.Conversation.ID}
	f.db.AddQueryHook(chatQueryHook{})
	gate := newChatQueryGate(t, false, 1, func(event *bun.QueryEvent) bool {
		return event.Operation() == "SELECT" && strings.Contains(event.Query, "AS service_session_id") && strings.Contains(event.Query, "cv.id IN")
	})
	var snapshot []inboxaction.ConversationResult
	done := make(chan error, 1)
	go func() {
		var err error
		snapshot, err = query.ReadByIDs(context.WithValue(ctx, chatQueryGateKey{}, gate), f.owner, ids, &mine)
		done <- err
	}()
	waitChatSignal(t, ctx, gate.reached)
	if _, err := conversationaction.NewTransferServiceSessionAction(f.db, agentrunaction.NewExecuteAction(f.db, nil, nil), agentrunaction.NewScheduler(servertask.New(f.db, serverconfig.NATSConfig{}))).Execute(ctx, f.owner, conversationaction.TransferServiceSessionInput{ConversationID: f.conversationID, AssigneeIdentityID: f.member.OrganizationIdentity.ID}); err != nil {
		t.Fatal(err)
	}
	gate.open()
	if err := waitChatResult(t, ctx, done); err != nil {
		t.Fatal(err)
	}
	if !snapshot[0].MatchesQuery || snapshot[0].Conversation.Customer.Assignee.IdentityID != f.owner.OrganizationIdentity.ID || snapshot[1].MatchesQuery || snapshot[1].Conversation.Direct == nil {
		t.Fatalf("mixed snapshot=%+v", snapshot)
	}
	current, err := query.ReadByIDs(ctx, f.owner, ids, &mine)
	if err != nil || current[0].MatchesQuery || current[0].Conversation.Customer.Assignee.IdentityID != f.member.OrganizationIdentity.ID {
		t.Fatalf("transferred=%+v err=%v", current, err)
	}
	if _, err := conversationaction.NewCloseServiceSessionAction(f.db, agentrunaction.NewExecuteAction(f.db, nil, nil)).Execute(ctx, f.member, f.conversationID); err != nil {
		t.Fatal(err)
	}
	closed := inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewCoworkers, ServiceStatus: domain.ServiceSessionStatusClosed}
	current, err = query.ReadByIDs(ctx, f.owner, ids, &closed)
	if err != nil || !current[0].MatchesQuery || current[0].Conversation.Customer.ServiceSessionStatus != domain.ServiceSessionStatusClosed {
		t.Fatalf("closed=%+v err=%v", current, err)
	}
	if _, err := query.ReadByIDs(ctx, f.owner, []string{"bad-id"}, &closed); !errors.Is(err, inboxaction.ErrQueryInvalid) {
		t.Fatalf("invalid ID=%v", err)
	}
	empty, err := query.ReadByIDs(ctx, f.owner, nil, &closed)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
}
