//go:build server

package integrationtest

import (
	"context"
	"errors"
	"testing"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
)

// loadServiceSessionQueue 读取客户会话当前处理周期的负责人与所属队列。
func loadServiceSessionQueue(t *testing.T, f customerReadFixture, conversationID string) (*string, *string) {
	t.Helper()
	session := &servermodels.ServiceSession{}
	if err := f.db.NewSelect().Model(session).
		Join("JOIN customer_conversations AS cc ON cc.current_service_session_id = ss.id AND cc.organization_id = ss.organization_id").
		Where("ss.organization_id = ? AND cc.conversation_id = ?", f.owner.Organization.ID, conversationID).
		Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	return session.AssigneeIdentityID, session.TeamID
}

// queueConversationIDs 按队列筛选读取「待分配」视图中的会话编号与所属团队名称。
func queueConversationIDs(t *testing.T, f customerReadFixture, filter domain.CustomerQueueFilter, teamID string) map[string]*string {
	t.Helper()
	page, _, err := inboxaction.NewLoadInboxQuery(f.db).Execute(context.Background(), f.owner, inboxaction.LoadInput{
		Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewQueue,
		QueueFilter: filter, QueueTeamID: teamID, ServiceStatus: domain.ServiceSessionStatusOpen,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := make(map[string]*string, len(page.Conversations))
	for _, row := range page.Conversations {
		found[row.ID] = row.Customer.TeamName
	}
	return found
}

// TestServiceSessionTeamQueue 验证转交到团队与公共队列、团队可用性判定、收件箱队列筛选和删除团队后的队列归属。
func TestServiceSessionTeamQueue(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	coordinator := newGroupAgentCoordinator(f.db)
	scheduler := agentrunaction.NewScheduler(servertask.New(f.db, serverconfig.NATSConfig{}))
	claim := conversationaction.NewClaimServiceSessionAction(f.db, coordinator)
	transfer := conversationaction.NewTransferServiceSessionAction(f.db, coordinator, scheduler)
	createTeam := teamaction.NewCreateTeamAction(f.db)

	staffed, err := createTeam.Execute(ctx, f.owner, teamaction.Input{Name: "有人团队"})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := createTeam.Execute(ctx, f.owner, teamaction.Input{Name: "空团队"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := teamaction.NewAddMembersAction(f.db).Execute(ctx, f.owner, staffed.ID, []teamaction.MemberIdentity{
		{IdentityType: domain.OrganizationIdentityTypeUser, IdentityID: f.member.OrganizationIdentity.ID},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := claim.Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatal(err)
	}
	_, err = transfer.Execute(ctx, f.owner, conversationaction.TransferServiceSessionInput{
		ConversationID: f.conversationID, TargetKind: domain.ServiceSessionTargetTeam, TeamID: empty.ID,
	})
	var conflict *conversationaction.ConflictError
	if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonTransferTeamUnavailable {
		t.Fatalf("转交给没有接待成员的团队 = %v", err)
	}

	if _, err := transfer.Execute(ctx, f.owner, conversationaction.TransferServiceSessionInput{
		ConversationID: f.conversationID, TargetKind: domain.ServiceSessionTargetTeam, TeamID: staffed.ID,
	}); err != nil {
		t.Fatalf("转交给团队 = %v", err)
	}
	assignee, teamID := loadServiceSessionQueue(t, f, f.conversationID)
	if assignee != nil || teamID == nil || *teamID != staffed.ID {
		t.Fatalf("转交到团队队列后归属 = %v, %v", assignee, teamID)
	}

	if name, ok := queueConversationIDs(t, f, domain.CustomerQueueFilterTeam, staffed.ID)[f.conversationID]; !ok || name == nil || *name != staffed.Name {
		t.Fatalf("团队队列筛选缺少会话或团队名称 = %v", name)
	}
	if _, ok := queueConversationIDs(t, f, domain.CustomerQueueFilterPublic, "")[f.conversationID]; ok {
		t.Fatal("公共队列筛选不应包含团队队列中的会话")
	}
	if name, ok := queueConversationIDs(t, f, domain.CustomerQueueFilterAll, "")[f.conversationID]; !ok || name == nil {
		t.Fatalf("全部队列筛选缺少会话或团队标签 = %v", name)
	}

	// 领取不改变队列，退回公共队列同时清空负责人与队列。
	if _, err := claim.Execute(ctx, f.member, f.conversationID); err != nil {
		t.Fatal(err)
	}
	if _, teamID := loadServiceSessionQueue(t, f, f.conversationID); teamID == nil || *teamID != staffed.ID {
		t.Fatalf("领取后队列 = %v", teamID)
	}
	if _, err := transfer.Execute(ctx, f.member, conversationaction.TransferServiceSessionInput{
		ConversationID: f.conversationID, TargetKind: domain.ServiceSessionTargetPublicQueue,
	}); err != nil {
		t.Fatalf("退回公共队列 = %v", err)
	}
	assignee, teamID = loadServiceSessionQueue(t, f, f.conversationID)
	if assignee != nil || teamID != nil {
		t.Fatalf("退回公共队列后归属 = %v, %v", assignee, teamID)
	}

	// 渠道路由到没有接待成员的团队时降级到公共队列。
	updateChannel := channelaction.NewUpdateMessageChannelAction(f.db)
	route := func(target channelaction.RoutingTarget) {
		t.Helper()
		if _, err := updateChannel.Execute(ctx, f.owner, f.channelID, channelaction.MessageChannelInput{
			Name: "客服未读测试", DefaultLocale: domain.LocaleChineseSimplified,
			NewConversationTarget: target,
			FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		}); err != nil {
			t.Fatal(err)
		}
	}
	newSession := func(externalID string) string {
		t.Helper()
		result, err := f.receive.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
			ChannelID: f.channelID, ExternalID: externalID, ClientMessageID: uuid.NewV7().String(), Body: "新访客消息",
		})
		if err != nil {
			t.Fatal(err)
		}
		return result.Conversation.ID
	}
	route(channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeTeam, ID: empty.ID})
	if _, teamID := loadServiceSessionQueue(t, f, newSession("web-session:11111111111111111111111111111111")); teamID != nil {
		t.Fatalf("路由到空团队应降级到公共队列，实际队列 = %v", *teamID)
	}
	route(channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeTeam, ID: staffed.ID})
	teamConversationID := newSession("web-session:22222222222222222222222222222222")
	if _, teamID := loadServiceSessionQueue(t, f, teamConversationID); teamID == nil || *teamID != staffed.ID {
		t.Fatalf("路由到有接待成员的团队，实际队列 = %v", teamID)
	}

	// 删除团队后其队列中的处理周期并入公共队列。
	route(channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	if err := teamaction.NewDeleteTeamAction(f.db).Execute(ctx, f.owner, staffed.ID); err != nil {
		t.Fatal(err)
	}
	if _, teamID := loadServiceSessionQueue(t, f, teamConversationID); teamID != nil {
		t.Fatalf("删除团队后队列 = %v", *teamID)
	}
}
