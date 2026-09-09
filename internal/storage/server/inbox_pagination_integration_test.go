//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/appservice"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

type inboxPaginationFixture struct {
	customerReadFixture
	internalIDs []string
	customerIDs map[domain.CustomerInboxView][]string
}

// newInboxPaginationFixture 用真实创建和发送入口建立四类各六十条会话。
func newInboxPaginationFixture(t *testing.T) inboxPaginationFixture {
	t.Helper()
	f := inboxPaginationFixture{customerReadFixture: newCustomerReadFixture(t), customerIDs: make(map[domain.CustomerInboxView][]string)}
	ctx := context.Background()
	provider := &servermodels.AIProvider{OrganizationID: f.owner.Organization.ID, Brand: "openai", Name: "分页模型", APIKey: "test", APIURL: "https://example.com/v1"}
	if _, err := f.db.NewInsert().Model(provider).Column("organization_id", "brand", "name", "api_key", "api_url").Returning("id").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	model := &servermodels.AIProviderModel{ProviderID: provider.ID, OrganizationID: f.owner.Organization.ID, Identifier: "test-model", Name: "分页模型", Type: "chat", InputModalities: json.RawMessage(`["text"]`), ContextWindow: 1000, MaxOutputTokens: 100}
	if _, err := f.db.NewInsert().Model(model).Column("provider_id", "organization_id", "identifier", "name", "model_type", "input_modalities", "context_window", "max_output_tokens").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	agent, err := agentaction.NewCreateAgentAction(f.db).Execute(ctx, f.owner, agentaction.CreateInput{DisplayName: "分页助手", RoleID: f.member.OrganizationIdentity.RoleID, Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{ProviderID: provider.ID, ModelIdentifier: model.Identifier, SystemInstruction: "测试分页"}}})
	if err != nil {
		t.Fatal(err)
	}
	tasks := servertask.New(f.db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	startAgent := conversationaction.NewSendFirstAgentTextMessageAction(f.db, agentrunaction.NewScheduler(tasks))
	views := []domain.CustomerInboxView{domain.CustomerInboxViewQueue, domain.CustomerInboxViewMine, domain.CustomerInboxViewCoworkers, domain.CustomerInboxViewClosed}
	for index := range 60 {
		peer, err := useraction.NewCreateUserAction(f.db).Execute(ctx, f.owner, useraction.CreateInput{DisplayName: fmt.Sprintf("分页成员 %d", index), Email: fmt.Sprintf("page%d@test.example", index), Password: "password123", RoleID: f.member.OrganizationIdentity.RoleID})
		if err != nil {
			t.Fatal(err)
		}
		direct, err := conversationaction.NewSendFirstDirectTextMessageAction(f.db).Execute(ctx, f.owner, conversationaction.FirstDirectTextMessageInput{TargetIdentityID: peer.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "单聊"})
		if err != nil {
			t.Fatal(err)
		}
		ai, err := startAgent.Execute(ctx, f.owner, conversationaction.FirstAgentTextMessageInput{ConversationID: uuid.NewV7().String(), AgentIdentityID: agent.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "AI 会话"})
		if err != nil {
			t.Fatal(err)
		}
		groupID := f.groupID
		customerID := f.conversationID
		if index > 0 {
			group, err := conversationaction.NewCreateGroupConversationAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationInput{Title: fmt.Sprintf("分页群 %d", index), MemberIdentityIDs: []string{f.member.OrganizationIdentity.ID}})
			if err != nil {
				t.Fatal(err)
			}
			groupID = group.ID
			customer, err := f.receive.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{ChannelID: f.channelID, ExternalID: "web-session:" + strings.ReplaceAll(uuid.NewV7().String(), "-", ""), ClientMessageID: uuid.NewV7().String(), Body: "访客会话"})
			if err != nil {
				t.Fatal(err)
			}
			customerID = customer.Conversation.ID
		}
		view := views[index%len(views)]
		if view != domain.CustomerInboxViewQueue {
			assignee := f.owner
			if view == domain.CustomerInboxViewCoworkers {
				assignee = f.member
			}
			if _, err := conversationaction.NewClaimServiceSessionAction(f.db, nil).Execute(ctx, assignee, customerID); err != nil {
				t.Fatal(err)
			}
			if view == domain.CustomerInboxViewClosed {
				if _, err := conversationaction.NewCloseServiceSessionAction(f.db, agentrunaction.NewExecuteAction(f.db, nil, nil)).Execute(ctx, assignee, customerID); err != nil {
					t.Fatal(err)
				}
			}
		}
		f.internalIDs = append(f.internalIDs, direct.Conversation.ID, ai.Conversation.ID, groupID)
		f.customerIDs[view] = append(f.customerIDs[view], customerID)
		// 每批四种类型共享微秒时间，定期插入空时间以覆盖完整空值分区。
		var activity *time.Time
		if index%7 != 0 {
			value := time.Date(2026, 9, 9, 0, 0, 0, 123000000, time.UTC).Add(time.Duration(index/2) * time.Microsecond)
			activity = &value
		}
		if _, err := f.db.NewUpdate().Model((*servermodels.Conversation)(nil)).Set("last_activity_at = ?", activity).Where("id IN (?)", bun.In([]string{direct.Conversation.ID, ai.Conversation.ID, groupID, customerID})).Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}
	// 批量造数后刷新数据库统计信息。
	if _, err := f.db.ExecContext(ctx, "ANALYZE"); err != nil {
		t.Fatal(err)
	}
	return f
}

// TestInboxPagination 验证全量 SQL 顺序、各筛选、重复请求和页外未读总数。
func TestInboxPagination(t *testing.T) {
	f := newInboxPaginationFixture(t)
	ctx := context.Background()
	f.send(t, f.member, "页外未读", false)
	// 将未读会话放在最后，确保首屏总数并非来自首屏行的加总。
	if _, err := f.db.NewUpdate().Model((*servermodels.Conversation)(nil)).Set("last_activity_at = NULL").Where("id = ?", f.groupID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		input inboxaction.LoadInput
		ids   []string
	}{
		{"all", inboxaction.LoadInput{}, append(slices.Clone(f.internalIDs), f.customerIDs[domain.CustomerInboxViewMine]...)},
		{"internal", inboxaction.LoadInput{Scope: domain.InboxScopeInternal, Limit: 17}, f.internalIDs},
	}
	for _, view := range []domain.CustomerInboxView{domain.CustomerInboxViewQueue, domain.CustomerInboxViewMine, domain.CustomerInboxViewCoworkers, domain.CustomerInboxViewClosed} {
		cases = append(cases, struct {
			name  string
			input inboxaction.LoadInput
			ids   []string
		}{string(view), inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: view, Limit: 4}, f.customerIDs[view]})
	}
	cases = append(cases, struct {
		name  string
		input inboxaction.LoadInput
		ids   []string
	}{"assignee", inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewCoworkers, AssigneeIdentityID: f.member.OrganizationIdentity.ID, Limit: 3}, f.customerIDs[domain.CustomerInboxViewCoworkers]})
	cases = append(cases, struct {
		name  string
		input inboxaction.LoadInput
		ids   []string
	}{"unavailable_assignee", inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewCoworkers, AssigneeIdentityID: uuid.NewV7().String()}, nil})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var expected []string
			if err := f.db.NewSelect().Table("conversations").Column("id").Where("id IN (?)", bun.In(tc.ids)).OrderExpr("last_activity_at DESC NULLS LAST, id DESC").Scan(ctx, &expected); err != nil {
				t.Fatal(err)
			}
			query := inboxaction.NewLoadInboxQuery(f.db)
			input := tc.input
			var actual []string
			for attempts := 0; ; attempts++ {
				if attempts > len(expected) {
					t.Fatal("pagination did not terminate")
				}
				page, counts, err := query.Execute(ctx, f.owner, input)
				if err != nil {
					t.Fatal(err)
				}
				if counts.Unread != 1 || counts.Attention != 1 {
					t.Fatalf("page counts=%+v", counts)
				}
				if attempts == 0 {
					repeated, _, err := query.Execute(ctx, f.owner, input)
					if err != nil || repeated.NextCursor != page.NextCursor || !slices.EqualFunc(repeated.Conversations, page.Conversations, func(a, b inboxaction.ConversationSummary) bool { return a.ID == b.ID }) {
						t.Fatalf("repeated page=%+v err=%v", repeated, err)
					}
					if input.Limit == 0 && len(page.Conversations) != min(50, len(expected)) {
						t.Fatalf("default page size=%d", len(page.Conversations))
					}
				}
				for _, row := range page.Conversations {
					actual = append(actual, row.ID)
				}
				if !page.HasMore {
					if page.NextCursor != "" {
						t.Fatal("terminal page returned cursor")
					}
					break
				}
				if page.NextCursor == "" || page.NextCursor == input.Cursor {
					t.Fatal("cursor did not advance")
				}
				input.Cursor = page.NextCursor
			}
			if !slices.Equal(actual, expected) {
				t.Fatalf("pages=%v expected=%v", actual, expected)
			}
		})
	}
}

// TestInboxPaginationBoundaries 验证游标行删除、失权、空尾页及应用服务错误语义。
func TestInboxPaginationBoundaries(t *testing.T) {
	for _, withTime := range []bool{false, true} {
		t.Run(fmt.Sprintf("activity=%t", withTime), func(t *testing.T) {
			f := newNavigationFixture(t)
			ctx := tenant.WithAccessHost(context.Background(), f.owner.Organization.AccessHost)
			ids := []string{f.groupID}
			for range 3 {
				group, err := conversationaction.NewCreateGroupConversationAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationInput{Title: "边界群", MemberIdentityIDs: []string{f.member.OrganizationIdentity.ID}})
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, group.ID)
			}
			if withTime {
				for index, id := range ids {
					activity := time.Date(2026, 9, 9, 0, 0, 0, 123456000, time.UTC).Add(time.Duration(index/2) * time.Microsecond)
					if _, err := f.db.NewUpdate().Model((*servermodels.Conversation)(nil)).Set("last_activity_at = ?", activity).Where("id = ?", id).Exec(ctx); err != nil {
						t.Fatal(err)
					}
				}
			}

			query := inboxaction.NewLoadInboxQuery(f.db)
			input := inboxaction.LoadInput{Scope: domain.InboxScopeInternal, Limit: 2}
			page, _, err := query.Execute(ctx, f.member, input)
			if err != nil || !page.HasMore {
				t.Fatalf("first page=%+v err=%v", page, err)
			}
			input.Cursor = page.NextCursor
			// 核验时间与空时间边界在会话删除后仍保留原始值。
			if _, err := f.db.NewDelete().Model((*servermodels.Conversation)(nil)).Where("id = ?", page.Conversations[1].ID).Exec(ctx); err != nil {
				t.Fatal(err)
			}
			next, _, err := query.Execute(ctx, f.member, input)
			if err != nil || len(next.Conversations) != 2 || next.HasMore || next.Conversations[1].ID != f.groupID {
				t.Fatalf("deleted boundary=%+v err=%v", next, err)
			}
			for _, row := range next.Conversations {
				if _, err := conversationaction.NewRemoveGroupConversationMemberAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationMemberInput{ConversationID: row.ID, MemberIdentityID: f.member.OrganizationIdentity.ID}); err != nil {
					t.Fatal(err)
				}
			}
			empty, _, err := query.Execute(ctx, f.member, input)
			if err != nil || empty.Conversations == nil || len(empty.Conversations) != 0 || empty.HasMore || empty.NextCursor != "" {
				t.Fatalf("empty tail=%+v err=%v", empty, err)
			}
			login, err := authaction.NewLoginAction(f.db).Execute(ctx, authaction.LoginInput{OrganizationID: f.owner.Organization.ID, Email: "member@navigation.test", Password: "password123"})
			if err != nil {
				t.Fatal(err)
			}
			backend := appservice.NewDirectBackend(f.db, nil, NewTenantResolver(f.db), nil, nil, nil)
			meta := appservice.RequestMeta{Token: login.Token}
			for _, request := range []appservice.LoadInboxInput{
				{Scope: appservice.InboxScopeCustomer, Cursor: input.Cursor},
				{Scope: appservice.InboxScopeInternal, Cursor: "invalid"},
			} {
				_, err := backend.LoadInbox(ctx, meta, request)
				var apiError *appservice.Error
				if !errors.As(err, &apiError) || apiError.Reason != "inbox_cursor_invalid" {
					t.Fatalf("cursor error=%v", err)
				}
			}
			_, _, err = query.Execute(ctx, f.owner, input)
			if !errors.Is(err, inboxaction.ErrCursorInvalid) {
				t.Fatalf("other user cursor=%v", err)
			}
			foreign := newNavigationFixture(t)
			_, _, err = query.Execute(ctx, foreign.owner, input)
			if !errors.Is(err, inboxaction.ErrCursorInvalid) {
				t.Fatalf("other organization cursor=%v", err)
			}
		})
	}
}
