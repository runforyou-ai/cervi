//go:build server

package integrationtest

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/domain"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	"github.com/runforyou-ai/cervi/internal/tenant"
)

// conversationIDs 返回列表结果的会话编号，用于比对筛选命中范围。
func conversationIDs(summaries []inboxaction.ConversationSummary) []string {
	ids := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		ids = append(ids, summary.ID)
	}
	return ids
}

// TestInboxChannelFilter 验证渠道筛选在排队和已关闭视图收敛，且停用渠道仍在候选与结果中。
func TestInboxChannelFilter(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	query := inboxaction.NewLoadInboxQuery(f.db)
	second, err := channelaction.NewCreateMessageChannelAction(f.db).Execute(ctx, f.owner, channelaction.CreateMessageChannelInput{
		Type: domain.ChannelTypeWebsite, Name: "渠道筛选测试", DefaultLocale: domain.LocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.receive.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: second.ID, ExternalID: "web-session:" + strings.ReplaceAll(uuid.NewV7().String(), "-", ""), ClientMessageID: uuid.NewV7().String(), Body: "另一渠道客户消息",
	})
	if err != nil {
		t.Fatal(err)
	}
	queue := inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewQueue}
	both, _, err := query.Execute(ctx, f.owner, queue)
	if err != nil || len(both.Conversations) != 2 {
		t.Fatalf("unfiltered=%v err=%v", conversationIDs(both.Conversations), err)
	}
	for channelID, expected := range map[string]string{f.channelID: f.conversationID, second.ID: other.Conversation.ID} {
		filtered := queue
		filtered.ChannelID = channelID
		page, _, err := query.Execute(ctx, f.owner, filtered)
		if err != nil || !slices.Equal(conversationIDs(page.Conversations), []string{expected}) {
			t.Fatalf("channel=%s got=%v err=%v", channelID, conversationIDs(page.Conversations), err)
		}
		// 筛选只决定列表资格，落选会话仍保留阅读资格。
		results, err := query.ReadByIDs(ctx, f.owner, []string{f.conversationID, other.Conversation.ID}, &filtered)
		if err != nil {
			t.Fatal(err)
		}
		for _, result := range results {
			if result.Conversation == nil || result.MatchesQuery != (result.ID == expected) {
				t.Fatalf("channel=%s eligibility=%+v", channelID, result)
			}
		}
	}
	// 领取并关闭一条会话，核对渠道条件在已关闭视图同样生效。
	if _, err := conversationaction.NewClaimServiceSessionAction(f.db, nil).Execute(ctx, f.owner, other.Conversation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewCloseServiceSessionAction(f.db, agentrunaction.NewExecuteAction(f.db, nil, nil)).Execute(ctx, f.owner, other.Conversation.ID); err != nil {
		t.Fatal(err)
	}
	closed := inboxaction.LoadInput{
		Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewMine,
		ServiceStatus: domain.ServiceSessionStatusClosed, ChannelID: second.ID,
	}
	closedPage, _, err := query.Execute(ctx, f.owner, closed)
	if err != nil || !slices.Equal(conversationIDs(closedPage.Conversations), []string{other.Conversation.ID}) {
		t.Fatalf("closed by channel=%v err=%v", conversationIDs(closedPage.Conversations), err)
	}
	closed.ChannelID = f.channelID
	closedPage, _, err = query.Execute(ctx, f.owner, closed)
	if err != nil || len(closedPage.Conversations) != 0 {
		t.Fatalf("other channel closed=%v err=%v", conversationIDs(closedPage.Conversations), err)
	}
	// 停用渠道只影响后续接收，历史会话仍按原渠道筛出，候选中保留该渠道。
	if _, err := channelaction.NewUpdateMessageChannelStatusAction(f.db).Execute(ctx, f.owner, second.ID, false); err != nil {
		t.Fatal(err)
	}
	closed.ChannelID = second.ID
	closedPage, _, err = query.Execute(ctx, f.owner, closed)
	if err != nil || !slices.Equal(conversationIDs(closedPage.Conversations), []string{other.Conversation.ID}) {
		t.Fatalf("disabled channel=%v err=%v", conversationIDs(closedPage.Conversations), err)
	}
	login, err := authaction.NewLoginAction(f.db).Execute(ctx, authaction.LoginInput{OrganizationID: f.owner.Organization.ID, Email: "owner@navigation.test", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	var accessHost string
	if err := f.db.NewSelect().Table("organizations").Column("access_host").Where("id = ?", f.owner.Organization.ID).Scan(ctx, &accessHost); err != nil {
		t.Fatal(err)
	}
	backend := appservice.NewDirectBackend(f.db, nil, serverstorage.NewTenantResolver(f.db), nil, nil, nil, nil)
	candidates, err := backend.ListInboxChannels(tenant.WithAccessHost(ctx, accessHost), appservice.RequestMeta{Token: login.Token})
	if err != nil || len(candidates.Channels) != 2 {
		t.Fatalf("channel candidates=%+v err=%v", candidates.Channels, err)
	}
	// 候选按渠道类型的既定顺序和名称排序，停用渠道保留在列表中。
	if candidates.Channels[0].Name != "客服未读测试" || candidates.Channels[1].ID != second.ID || candidates.Channels[1].Enabled {
		t.Fatalf("channel candidate order=%+v", candidates.Channels)
	}
}

// TestInboxKindFilter 验证会话类型筛选按范围收敛，并拒绝范围外的类型和条件。
func TestInboxKindFilter(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	query := inboxaction.NewLoadInboxQuery(f.db)
	direct, err := conversationaction.NewSendFirstDirectTextMessageAction(f.db).Execute(ctx, f.owner, conversationaction.FirstDirectTextMessageInput{
		TargetIdentityID: f.member.OrganizationIdentity.ID, ClientMessageID: uuid.NewV7().String(), Body: "内部单聊",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		input    inboxaction.LoadInput
		expected []string
	}{
		{"group", inboxaction.LoadInput{Scope: domain.InboxScopeInternal, Kinds: []domain.ConversationType{domain.ConversationTypeGroup}}, []string{f.groupID}},
		{"direct", inboxaction.LoadInput{Scope: domain.InboxScopeInternal, Kinds: []domain.ConversationType{domain.ConversationTypeDirect}}, []string{direct.Conversation.ID}},
		{"internal", inboxaction.LoadInput{Scope: domain.InboxScopeInternal}, []string{direct.Conversation.ID, f.groupID}},
		{"all_internal_kinds", inboxaction.LoadInput{Kinds: []domain.ConversationType{domain.ConversationTypeDirect, domain.ConversationTypeGroup}}, []string{direct.Conversation.ID, f.groupID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := query.Execute(ctx, f.owner, tc.input)
			ids := conversationIDs(page.Conversations)
			slices.Sort(ids)
			expected := slices.Clone(tc.expected)
			slices.Sort(expected)
			if err != nil || !slices.Equal(ids, expected) {
				t.Fatalf("got=%v want=%v err=%v", ids, expected, err)
			}
		})
	}
	for _, input := range []inboxaction.LoadInput{
		{Scope: domain.InboxScopeInternal, Kinds: []domain.ConversationType{domain.ConversationTypeCustomer}},
		{Scope: domain.InboxScopeCustomer, Kinds: []domain.ConversationType{domain.ConversationTypeGroup}},
		{Scope: domain.InboxScopeCustomer, ServiceStatus: "archived"},
		{Scope: domain.InboxScopeCustomer, ChannelID: "bad-id"},
	} {
		if _, _, err := query.Execute(ctx, f.owner, input); !errors.Is(err, inboxaction.ErrQueryInvalid) {
			t.Fatalf("accepted invalid filter=%+v err=%v", input, err)
		}
	}
	// 按 ID 读取时类型筛选只决定列表资格，落选会话仍可阅读。
	groupOnly := inboxaction.LoadInput{Scope: domain.InboxScopeInternal, Kinds: []domain.ConversationType{domain.ConversationTypeGroup}}
	results, err := query.ReadByIDs(ctx, f.owner, []string{f.groupID, direct.Conversation.ID}, &groupOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.Conversation == nil || result.MatchesQuery != (result.ID == f.groupID) {
			t.Fatalf("kind eligibility=%+v", result)
		}
	}
	// 勾满当前范围全部类型等同不限类型，游标身份也一致。
	full, _, err := query.Execute(ctx, f.owner, inboxaction.LoadInput{
		Scope: domain.InboxScopeInternal,
		Kinds: []domain.ConversationType{domain.ConversationTypeDirect, domain.ConversationTypeGroup, domain.ConversationTypeAgent},
	})
	unlimited, _, err2 := query.Execute(ctx, f.owner, inboxaction.LoadInput{Scope: domain.InboxScopeInternal})
	if err != nil || err2 != nil || !slices.Equal(conversationIDs(full.Conversations), conversationIDs(unlimited.Conversations)) || full.EndCursor != unlimited.EndCursor {
		t.Fatalf("full=%v unlimited=%v err=%v %v", conversationIDs(full.Conversations), conversationIDs(unlimited.Conversations), err, err2)
	}
	// 客户队列条件在其他范围一律按空条件读取。
	carried, _, err := query.Execute(ctx, f.owner, inboxaction.LoadInput{
		Scope: domain.InboxScopeInternal, CustomerView: domain.CustomerInboxViewMine,
		AssigneeIdentityID: f.member.OrganizationIdentity.ID, ChannelID: f.channelID, ServiceStatus: domain.ServiceSessionStatusClosed,
	})
	plain, _, err2 := query.Execute(ctx, f.owner, inboxaction.LoadInput{Scope: domain.InboxScopeInternal})
	if err != nil || err2 != nil || !slices.Equal(conversationIDs(carried.Conversations), conversationIDs(plain.Conversations)) {
		t.Fatalf("carried=%v plain=%v err=%v %v", conversationIDs(carried.Conversations), conversationIDs(plain.Conversations), err, err2)
	}
}
