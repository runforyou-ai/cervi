//go:build server

package integrationtest

import (
	"context"
	"testing"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	aiperformanceaction "github.com/runforyou-ai/cervi/internal/actions/aiperformance"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	"github.com/runforyou-ai/cervi/internal/actions/knowledgegap"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
	"uuid"
)

// testAIPerformanceReport 验证 AI 表现报表按小结的是否解决统计解决情况，只把 AI 员工关闭且无真人参与的已解决周期计入独立解决，排除无实质诉求的周期，按已关闭周期统计转人工原因，并统计待处理的待补知识。
func testAIPerformanceReport(t *testing.T, db *bun.DB, identity *servermodels.Identity, providerID, modelID string) {
	ctx := context.Background()
	tasks := newTestTasks(db)
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := tasks.Registry().RegisterJSON(deliveryaction.SendActionName, func(context.Context, deliveryaction.Input) error { return nil }); err != nil {
		t.Fatal(err)
	}
	f := handoffFixture{db: db, identity: identity, tasks: tasks, providerID: providerID, modelID: modelID}
	disableAutoAssignment(t, db, identity.Organization.ID)
	agent := f.newAgent(t, "表现报表客服")
	channelID := f.newChannel(t, agent.IdentityID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	// closeSession 把周期按指定结束方式关闭，并写入访客评价与小结的是否解决。
	closeSession := func(sessionID string, reason domain.ServiceSessionCloseReason, rating, resolved *bool) {
		if _, err := db.NewUpdate().Table("service_sessions").
			Set("status = ?, close_reason = ?, closed_at = now(), rating_resolved = ?, resolved = ?", domain.ServiceSessionStatusClosed, reason, rating, resolved).
			Where("id = ?", sessionID).Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}

	// currentSession 读取客户会话当前的客服周期编号。
	currentSession := func(conversationID string) string {
		var sessionID string
		if err := db.NewSelect().Table("service_conversations").Column("current_service_session_id").
			Where("conversation_id = ?", conversationID).Scan(ctx, &sessionID); err != nil {
			t.Fatal(err)
		}
		return sessionID
	}

	resolvedInput := visitorInput(channelID, "")
	resolved := f.receive(t, &resolvedInput, "问题解决了，谢谢")
	resolveRun := f.executeQueuedRun(t, resolved.Conversation.ID, resolutionRuntime("不客气", agentruntime.TerminalDecision{Kind: domain.AgentRunOutcomeResolve}, nil, nil))
	assertAgentClosed(t, db, loadSession(t, db, resolveRun.ScopeID), agent.IdentityID, domain.ServiceSessionCloseAIResolved)

	handoffInput := visitorInput(channelID, "")
	handedOff := f.receive(t, &handoffInput, "海外仓发货要几天")
	handoffRun := f.executeQueuedRun(t, handedOff.Conversation.ID, handoffRuntime("资料里没有海外仓时效", nil))
	unresolved := false
	closeSession(handoffRun.ScopeID, domain.ServiceSessionCloseAIResolved, &unresolved, &unresolved)
	handoffSession := loadSession(t, db, handoffRun.ScopeID)
	if err := knowledgegap.RecordClosed(ctx, db, tasks, &handoffSession); err != nil {
		t.Fatal(err)
	}

	// 无实质诉求的周期不计入报表。
	greetingInput := visitorInput(channelID, "")
	greeting := f.receive(t, &greetingInput, "你好")
	greetingSessionID := currentSession(greeting.Conversation.ID)
	closeSession(greetingSessionID, domain.ServiceSessionCloseCustomerUnresponsive, nil, nil)
	if _, err := db.NewUpdate().Table("service_sessions").Set("summary_status = ?", domain.ServiceSessionSummaryNoRequest).
		Where("id = ?", greetingSessionID).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	scope := aiperformanceaction.Input{Days: 7, ChannelID: channelID}
	overview, err := aiperformanceaction.NewOverviewQuery(db).Execute(ctx, identity, scope)
	if err != nil {
		t.Fatal(err)
	}
	summary := overview.Summary
	if summary.Closed != 2 || summary.Resolved != 1 || summary.Unresolved != 1 || summary.AIOnly != 1 || summary.AIResolved != 1 || summary.AIUnresolved != 0 ||
		summary.HandedOff != 1 || summary.CloseAIResolved != 2 || summary.CustomerUnresponsive != 0 || summary.Manual != 0 ||
		summary.Rated != 1 || summary.RatedResolved != 0 || overview.KnowledgeGapTotal != 1 {
		t.Fatalf("overview = %+v", overview)
	}
	if len(overview.HandoffReasons) != 1 || overview.HandoffReasons[0].Reason != string(domain.AgentHandoffReasonKnowledgeGap) || overview.HandoffReasons[0].Count != 1 {
		t.Fatalf("handoff reasons = %+v", overview.HandoffReasons)
	}
	breakdowns := aiperformanceaction.NewBreakdownQuery(db)
	channels, err := breakdowns.Execute(ctx, identity, aiperformanceaction.BreakdownInput{Input: scope, Dimension: domain.AIPerformanceDimensionChannel})
	if err != nil || channels.Total != 1 || len(channels.Rows) != 1 || *channels.Rows[0].ID != channelID || channels.Rows[0].Closed != 2 || channels.Rows[0].Resolved != 1 || channels.Rows[0].AIResolved != 1 {
		t.Fatalf("channels = %+v, error = %v", channels, err)
	}
	categories, err := breakdowns.Execute(ctx, identity, aiperformanceaction.BreakdownInput{Input: scope, Dimension: domain.AIPerformanceDimensionCategory})
	if err != nil || categories.Total != 1 || len(categories.Rows) != 1 || categories.Rows[0].ID != nil || categories.Rows[0].Closed != 2 {
		t.Fatalf("categories = %+v, error = %v", categories, err)
	}
	// 停用 AI 员工把其负责的周期退回队列，记为 AI 员工不可用的转人工。
	retired := f.newAgent(t, "表现报表停用客服")
	retiredChannelID := f.newChannel(t, retired.IdentityID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	returnedInput := visitorInput(retiredChannelID, "")
	returned := f.receive(t, &returnedInput, "订单什么时候到")
	returnedSessionID := currentSession(returned.Conversation.ID)
	if _, err := agentaction.NewUpdateStatusAction(db, testServiceSessionReturner(db)).Execute(ctx, identity, retired.ID, domain.UserStatusInactive); err != nil {
		t.Fatal(err)
	}
	closeSession(returnedSessionID, domain.ServiceSessionCloseManual, nil, nil)
	overview, err = aiperformanceaction.NewOverviewQuery(db).Execute(ctx, identity, aiperformanceaction.Input{Days: 7, ChannelID: retiredChannelID})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Summary.Closed != 1 || overview.Summary.HandedOff != 1 || overview.Summary.Manual != 1 || overview.Summary.Resolved != 0 || overview.Summary.Unresolved != 0 || overview.KnowledgeGapTotal != 0 ||
		len(overview.HandoffReasons) != 1 || overview.HandoffReasons[0].Reason != string(domain.AgentHandoffReasonAgentUnavailable) || overview.HandoffReasons[0].Count != 1 {
		t.Fatalf("returned overview = %+v", overview)
	}

	// 真人对客回复过的已解决周期计入已解决，不计入独立解决。
	humanChannelID := f.newChannel(t, identity.OrganizationIdentity.ID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	repliedInput := visitorInput(humanChannelID, "")
	replied := f.receive(t, &repliedInput, "怎么修改收货地址")
	if _, err := conversationaction.NewSendServiceTextMessageAction(db, tasks).Execute(ctx, identity, conversationaction.ServiceTextMessageInput{
		ConversationID: replied.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "我来帮您修改",
	}); err != nil {
		t.Fatal(err)
	}
	resolvedByHuman := true
	closeSession(currentSession(replied.Conversation.ID), domain.ServiceSessionCloseAIResolved, nil, &resolvedByHuman)
	overview, err = aiperformanceaction.NewOverviewQuery(db).Execute(ctx, identity, aiperformanceaction.Input{Days: 7, ChannelID: humanChannelID})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Summary.Closed != 1 || overview.Summary.CloseAIResolved != 1 || overview.Summary.Resolved != 1 ||
		overview.Summary.AIOnly != 0 || overview.Summary.AIResolved != 0 || overview.Summary.HandedOff != 0 {
		t.Fatalf("human replied overview = %+v", overview.Summary)
	}

	// 真人未对客回复直接关闭的已解决周期计入已解决，不计入独立处理。
	closedByHumanChannelID := f.newChannel(t, identity.OrganizationIdentity.ID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	manualInput := visitorInput(closedByHumanChannelID, "")
	manual := f.receive(t, &manualInput, "发票怎么开")
	manualSessionID := currentSession(manual.Conversation.ID)
	if _, err := conversationaction.NewCloseServiceSessionAction(db, testServiceSessionReturner(db), tasks).Execute(ctx, identity, manual.Conversation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewUpdate().Table("service_sessions").Set("resolved = true").Where("id = ?", manualSessionID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	overview, err = aiperformanceaction.NewOverviewQuery(db).Execute(ctx, identity, aiperformanceaction.Input{Days: 7, ChannelID: closedByHumanChannelID})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Summary.Closed != 1 || overview.Summary.Manual != 1 || overview.Summary.Resolved != 1 || overview.Summary.AIOnly != 0 || overview.Summary.AIResolved != 0 {
		t.Fatalf("closed by human overview = %+v", overview.Summary)
	}

	// 全部渠道不按渠道过滤，包含上述四个渠道。
	overview, err = aiperformanceaction.NewOverviewQuery(db).Execute(ctx, identity, aiperformanceaction.Input{Days: 7})
	if err != nil || overview.Summary.Closed < 5 {
		t.Fatalf("all channels overview = %+v, error = %v", overview, err)
	}
	if channels, err = breakdowns.Execute(ctx, identity, aiperformanceaction.BreakdownInput{Input: aiperformanceaction.Input{Days: 7}, Dimension: domain.AIPerformanceDimensionChannel}); err != nil || channels.Total < 4 {
		t.Fatalf("all channels breakdown = %+v, error = %v", channels, err)
	}
}
