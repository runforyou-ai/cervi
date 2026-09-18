//go:build server

package integrationtest

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// testServiceSessionHandoff 创建管理操作交接客服周期所用的运行协调器。
func testServiceSessionHandoff(db *bun.DB) *agentrunaction.ExecuteAction {
	tasks := servertask.New(db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(deliveryaction.SendActionName, func(context.Context, deliveryaction.Input) error { return nil }); err != nil {
		panic(err)
	}
	return agentrunaction.NewExecuteAction(db, tasks, nil, testAttachmentReader(db), nil)
}

// handoffFixture 保存转人工集成测试共用的企业身份、任务运行时与客服角色。
type handoffFixture struct {
	db         *bun.DB
	identity   *servermodels.Identity
	tasks      *servertask.Runtime
	roleID     string
	providerID string
	modelID    string
}

// newAgent 创建一个客服角色的 AI 员工。
func (f handoffFixture) newAgent(t *testing.T, name string) *agentaction.Agent {
	t.Helper()
	created, err := agentaction.NewCreateAgentAction(f.db).Execute(context.Background(), f.identity, agentaction.CreateInput{
		DisplayName: name, RoleID: f.roleID,
		Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{ProviderID: f.providerID, ModelIdentifier: f.modelID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return created
}

// newChannel 创建初始路由到指定 AI 员工的网站渠道并返回渠道编号。
func (f handoffFixture) newChannel(t *testing.T, agentIdentityID string, fallback channelaction.RoutingTarget) string {
	t.Helper()
	channel, err := channelaction.NewCreateMessageChannelAction(f.db).Execute(context.Background(), f.identity, channelaction.CreateMessageChannelInput{
		Type: domain.ChannelTypeWebsite, Name: "转人工验证", DefaultLocale: domain.LocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeMember, ID: agentIdentityID},
		FallbackTarget:        fallback,
	})
	if err != nil {
		t.Fatal(err)
	}
	return channel.ID
}

// visitorInput 为新访客构造一条网站消息。
func visitorInput(channelID, body string) conversationaction.WebsiteCustomerTextMessageInput {
	return conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: channelID, ExternalID: "web-session:" + strings.ReplaceAll(uuid.NewV7().String(), "-", ""),
		ClientMessageID: uuid.NewV7().String(), Body: body,
	}
}

// receive 写入访客消息并返回结果，后续消息沿用同一会话。
func (f handoffFixture) receive(t *testing.T, input *conversationaction.WebsiteCustomerTextMessageInput, body string) conversationaction.ReceiveWebsiteCustomerMessageResult {
	t.Helper()
	input.ClientMessageID, input.Body = uuid.NewV7().String(), body
	result, err := conversationaction.NewReceiveWebsiteCustomerMessageAction(f.db, agentrunaction.NewScheduler(f.tasks)).Execute(context.Background(), *input)
	if err != nil {
		t.Fatal(err)
	}
	input.ConversationID = &result.Conversation.ID
	return result
}

// queuedRun 读取会话中排队的运行。
func (f handoffFixture) queuedRun(t *testing.T, conversationID string) servermodels.AgentRun {
	t.Helper()
	run := servermodels.AgentRun{}
	if err := f.db.NewSelect().Model(&run).Where("agr.conversation_id = ? AND agr.status = ?", conversationID, domain.AgentRunStatusQueued).Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	return run
}

// handoffRuntime 认领首批输入后按需执行插入动作，再返回模型给出的转人工决定。
func handoffRuntime(content, reasonText string, during func()) *testAgentRuntime {
	return &testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		triggers, err := feed.Peek(ctx, 0)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		claimed, err := feed.Claim(ctx, triggers[len(triggers)-1].Seq)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		if during != nil {
			during()
		}
		return agentruntime.RunResult{Content: content, EndSeq: claimed.EndSeq, Decision: agentruntime.TerminalDecision{
			Kind: domain.AgentRunOutcomeHandoff, Reason: domain.AgentHandoffReasonModelRequested, ReasonText: reasonText,
		}}, nil
	}}
}

// handoffEvents 读取会话中的转人工系统事件。
func handoffEvents(t *testing.T, db *bun.DB, conversationID string) []domain.ServiceSessionHandedOffEvent {
	t.Helper()
	var messages []servermodels.Message
	if err := db.NewSelect().Model(&messages).
		Where("msg.conversation_id = ? AND msg.system_event_type = ?", conversationID, domain.ConversationSystemEventServiceSessionHandedOff).
		OrderExpr("msg.message_seq").Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	events := make([]domain.ServiceSessionHandedOffEvent, 0, len(messages))
	for _, message := range messages {
		if message.Visibility != string(domain.MessageVisibilityInternalOnly) || message.SenderParticipantID != nil {
			t.Fatalf("handoff event message = %+v", message)
		}
		event := domain.ServiceSessionHandedOffEvent{}
		if err := json.Unmarshal(message.SystemEventPayload, &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	return events
}

// loadSession 读取客服周期的当前状态。
func loadSession(t *testing.T, db *bun.DB, sessionID string) servermodels.ServiceSession {
	t.Helper()
	session := servermodels.ServiceSession{}
	if err := db.NewSelect().Model(&session).Where("ss.id = ?", sessionID).Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	return session
}

// testAgentHandoffs 验证 AI 客服转人工的去向、并发边界、幂等、管理操作交接与资格变更互斥。
func testAgentHandoffs(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string) {
	tasks := servertask.New(db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	f := handoffFixture{db: db, identity: identity, tasks: tasks, roleID: roleID, providerID: providerID, modelID: modelID}
	t.Run("主动转人工后转回同一 AI", func(t *testing.T) { testModelHandoffRoundTrip(t, f) })
	t.Run("失败路由去向", func(t *testing.T) { testHandoffTargets(t, f) })
	t.Run("人工接管与交接先后", func(t *testing.T) { testHandoffCommitOrder(t, f) })
	t.Run("停用与改角色交接", func(t *testing.T) { testManagementHandoff(t, f) })
	t.Run("入站路由与资格变更交错", func(t *testing.T) { testInboundRoutingVersusEligibility(t, f) })
	t.Run("入站发现负责人失效", func(t *testing.T) { testInboundUnavailableAssignee(t, f) })
	t.Run("Telegram 主动转人工", func(t *testing.T) { testTelegramModelHandoff(t, f) })
	t.Run("渠道编辑与停用交错", func(t *testing.T) { testChannelEditVersusDeactivation(t, f) })
	t.Run("成员操作客服周期事件", func(t *testing.T) { testServiceSessionOperationEvents(t, f) })
	t.Run("Telegram 入站失效交接与运行收尾交错", func(t *testing.T) { testTelegramInboundHandoffVersusRunFailure(t, f) })
}

// testModelHandoffRoundTrip 验证最终认领后到达的消息随交接结算，人工转回 AI 后只处理新消息。
func testModelHandoffRoundTrip(t *testing.T, f handoffFixture) {
	ctx := context.Background()
	agent := f.newAgent(t, "转人工客服")
	channelID := f.newChannel(t, agent.IdentityID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	input := visitorInput(channelID, "")
	first := f.receive(t, &input, "我要退款")
	run := f.queuedRun(t, first.Conversation.ID)
	runtime := handoffRuntime("马上为您转接", "客户要求退款", func() { f.receive(t, &input, "还在吗") })
	executor := agentrunaction.NewExecuteAction(f.db, f.tasks, runtime, testAttachmentReader(f.db), nil)
	for range 2 {
		if err := executor.Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.db.NewSelect().Model(&run).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if run.Status != string(domain.AgentRunStatusSucceeded) || run.Outcome == nil || *run.Outcome != string(domain.AgentRunOutcomeHandoff) ||
		run.OutcomeReason == nil || *run.OutcomeReason != string(domain.AgentHandoffReasonModelRequested) ||
		run.InputEndSeq == nil || *run.InputEndSeq != 1 || run.HandoffSettledSeq == nil || *run.HandoffSettledSeq != 2 || run.ResponseMessageID == nil {
		t.Fatalf("handoff run = %+v", run)
	}
	lane := servermodels.AgentLane{}
	if err := f.db.NewSelect().Model(&lane).Where("al.id = ?", run.LaneID).Scan(ctx); err != nil || lane.DesiredSeq != 2 || lane.ProcessedSeq != 2 {
		t.Fatalf("lane = %+v, error = %v", lane, err)
	}
	session := loadSession(t, f.db, run.ScopeID)
	if session.AssigneeIdentityID != nil || session.TeamID != nil || session.Status != string(domain.ServiceSessionStatusOpen) {
		t.Fatalf("session = %+v", session)
	}
	events := handoffEvents(t, f.db, first.Conversation.ID)
	if len(events) != 1 || events[0].Target.Kind != domain.ServiceSessionTargetPublicQueue || events[0].ReasonText != "客户要求退款" ||
		events[0].FromDisplayName != "转人工客服" || events[0].AgentRunID == nil || *events[0].AgentRunID != run.ID {
		t.Fatalf("events = %+v", events)
	}
	if active, err := f.db.NewSelect().Model((*servermodels.AgentRun)(nil)).Where("agr.conversation_id = ? AND agr.status = ?", first.Conversation.ID, domain.AgentRunStatusQueued).Exists(ctx); err != nil || active {
		t.Fatalf("queued run after handoff = %t, error = %v", active, err)
	}
	// 访客只看到两条消息和对客通知，内部原因不外露。
	visible, err := conversationaction.NewListWebsiteMessagesQuery(f.db).Execute(ctx, conversationaction.MessageHistoryInput{ChannelID: channelID, ExternalID: input.ExternalID, ConversationID: first.Conversation.ID})
	if err != nil || len(visible.Messages) != 3 || visible.Messages[2].ID != *run.ResponseMessageID || visible.Messages[2].Body != "马上为您转接" {
		t.Fatalf("visitor messages = %+v, error = %v", visible, err)
	}
	for _, message := range visible.Messages {
		if strings.Contains(message.Body, "客户要求退款") {
			t.Fatalf("internal reason leaked: %+v", message)
		}
	}
	// 成员领取后转回同一 AI，新消息只触发新输入。
	coordinator := testServiceSessionHandoff(f.db)
	if _, err := conversationaction.NewClaimServiceSessionAction(f.db, coordinator).Execute(ctx, f.identity, first.Conversation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewTransferServiceSessionAction(f.db, coordinator, agentrunaction.NewScheduler(f.tasks)).Execute(ctx, f.identity, conversationaction.TransferServiceSessionInput{
		ConversationID: first.Conversation.ID, AssigneeIdentityID: agent.IdentityID,
	}); err != nil {
		t.Fatal(err)
	}
	f.receive(t, &input, "新的问题")
	next := f.queuedRun(t, first.Conversation.ID)
	reply := &testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		triggers, err := feed.Peek(ctx, 0)
		if err != nil || len(triggers) != 1 || triggers[0].Seq != 3 {
			t.Fatalf("triggers after handoff = %+v, error = %v", triggers, err)
		}
		claimed, err := feed.Claim(ctx, triggers[0].Seq)
		return agentruntime.RunResult{Content: "新问题的回答", EndSeq: claimed.EndSeq}, err
	}}
	if err := agentrunaction.NewExecuteAction(f.db, f.tasks, reply, testAttachmentReader(f.db), nil).Execute(ctx, agentrunaction.RunInput{RunID: next.ID}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.NewSelect().Model(&next).WherePK().Scan(ctx); err != nil || next.Status != string(domain.AgentRunStatusSucceeded) ||
		next.InputStartSeq != 3 || next.Outcome == nil || *next.Outcome != string(domain.AgentRunOutcomeReply) {
		t.Fatalf("next run = %+v, error = %v", next, err)
	}
}

// testHandoffTargets 验证转人工只按失败路由解析人工去向：团队、真人、公共队列，目标为 AI 或无效时进入公共队列。
func testHandoffTargets(t *testing.T, f handoffFixture) {
	ctx := context.Background()
	agent := f.newAgent(t, "去向验证客服")
	fallbackAgent := f.newAgent(t, "失败路由 AI")
	team, err := teamaction.NewCreateTeamAction(f.db).Execute(ctx, f.identity, teamaction.Input{Name: "售后组 " + uuid.NewV7().String()[:8]})
	if err != nil {
		t.Fatal(err)
	}
	human, err := useraction.NewCreateUserAction(f.db).Execute(ctx, f.identity, useraction.CreateInput{
		DisplayName: "人工客服", Email: "handoff-" + uuid.NewV7().String()[:8] + "@handoff.test", Password: "password123", RoleID: f.roleID,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name         string
		fallback     channelaction.RoutingTarget
		invalid      bool
		wantKind     domain.ServiceSessionTargetKind
		wantTeam     *string
		wantAssignee *string
	}{
		{name: "团队", fallback: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeTeam, ID: team.ID}, wantKind: domain.ServiceSessionTargetTeam, wantTeam: &team.ID},
		{name: "真人", fallback: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeMember, ID: human.IdentityID}, wantKind: domain.ServiceSessionTargetMember, wantAssignee: &human.IdentityID},
		{name: "AI 员工", fallback: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeMember, ID: fallbackAgent.IdentityID}, wantKind: domain.ServiceSessionTargetPublicQueue},
		{name: "无效目标", fallback: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeTeam, ID: team.ID}, invalid: true, wantKind: domain.ServiceSessionTargetPublicQueue},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			channelID := f.newChannel(t, agent.IdentityID, scenario.fallback)
			if scenario.invalid {
				if _, err := f.db.NewUpdate().Model((*servermodels.Channel)(nil)).Set("fallback_routing_target_id = ?", uuid.NewV7().String()).Where("id = ?", channelID).Exec(ctx); err != nil {
					t.Fatal(err)
				}
			}
			input := visitorInput(channelID, "")
			first := f.receive(t, &input, "需要人工")
			run := f.queuedRun(t, first.Conversation.ID)
			if err := agentrunaction.NewExecuteAction(f.db, f.tasks, handoffRuntime("", "无法确认", nil), testAttachmentReader(f.db), nil).Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
				t.Fatal(err)
			}
			session := loadSession(t, f.db, run.ScopeID)
			if (scenario.wantTeam == nil) != (session.TeamID == nil) || (scenario.wantTeam != nil && *session.TeamID != *scenario.wantTeam) ||
				(scenario.wantAssignee == nil) != (session.AssigneeIdentityID == nil) ||
				(scenario.wantAssignee != nil && (*session.AssigneeIdentityID != *scenario.wantAssignee || session.AssignedAt == nil)) {
				t.Fatalf("session = %+v", session)
			}
			events := handoffEvents(t, f.db, first.Conversation.ID)
			if len(events) != 1 || events[0].Target.Kind != scenario.wantKind ||
				(scenario.wantKind == domain.ServiceSessionTargetTeam && (events[0].Target.TeamName == nil || *events[0].Target.TeamName != team.Name)) ||
				(scenario.wantKind == domain.ServiceSessionTargetMember && (events[0].Target.DisplayName == nil || *events[0].Target.DisplayName != "人工客服")) {
				t.Fatalf("events = %+v", events)
			}
			// 模型未提供说明时使用渠道语言的内置通知。
			notice := servermodels.Message{}
			if err := f.db.NewSelect().Model(&notice).Where("msg.idempotency_key = ?", "agent:"+run.ID).Scan(ctx); err != nil || !strings.Contains(notice.Body, "人工客服") {
				t.Fatalf("notice = %+v, error = %v", notice, err)
			}
		})
	}
}

// testHandoffCommitOrder 验证人工接管先提交时抑制迟到的转人工结果，AI 交接先提交时人工基于新负责人继续操作。
func testHandoffCommitOrder(t *testing.T, f handoffFixture) {
	ctx := context.Background()
	agent := f.newAgent(t, "先后验证客服")
	channelID := f.newChannel(t, agent.IdentityID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	coordinator := testServiceSessionHandoff(f.db)

	input := visitorInput(channelID, "")
	first := f.receive(t, &input, "人工先接管")
	run := f.queuedRun(t, first.Conversation.ID)
	takeover := handoffRuntime("", "无法确认", func() {
		if _, err := conversationaction.NewClaimServiceSessionAction(f.db, coordinator).Execute(ctx, f.identity, first.Conversation.ID); err != nil {
			t.Fatal(err)
		}
	})
	if err := agentrunaction.NewExecuteAction(f.db, f.tasks, takeover, testAttachmentReader(f.db), nil).Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.NewSelect().Model(&run).WherePK().Scan(ctx); err != nil || run.Status != string(domain.AgentRunStatusCancelled) || run.Outcome != nil || run.ResponseMessageID != nil {
		t.Fatalf("suppressed run = %+v, error = %v", run, err)
	}
	session := loadSession(t, f.db, run.ScopeID)
	if session.AssigneeIdentityID == nil || *session.AssigneeIdentityID != f.identity.OrganizationIdentity.ID || len(handoffEvents(t, f.db, first.Conversation.ID)) != 0 {
		t.Fatalf("session after takeover = %+v", session)
	}

	input = visitorInput(channelID, "")
	second := f.receive(t, &input, "AI 先转人工")
	run = f.queuedRun(t, second.Conversation.ID)
	if err := agentrunaction.NewExecuteAction(f.db, f.tasks, handoffRuntime("", "无法确认", nil), testAttachmentReader(f.db), nil).Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	claimed, err := conversationaction.NewClaimServiceSessionAction(f.db, coordinator).Execute(ctx, f.identity, second.Conversation.ID)
	if err != nil || claimed.Assignee == nil || claimed.Assignee.IdentityID != f.identity.OrganizationIdentity.ID || len(handoffEvents(t, f.db, second.Conversation.ID)) != 1 {
		t.Fatalf("claim after handoff = %+v, error = %v", claimed, err)
	}
}

// testManagementHandoff 验证停用 AI 员工与改为非客服角色时，其负责的开放周期连同在途运行一并交给人工。
func testManagementHandoff(t *testing.T, f handoffFixture) {
	ctx := context.Background()
	for _, change := range []string{"停用", "改角色"} {
		t.Run(change, func(t *testing.T) {
			agent := f.newAgent(t, change+"客服")
			channelID := f.newChannel(t, agent.IdentityID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
			// 一个周期有在途运行，另一个周期已由 AI 回答完毕、没有在途运行。
			pendingInput := visitorInput(channelID, "")
			pending := f.receive(t, &pendingInput, "等待回答")
			pendingRun := f.queuedRun(t, pending.Conversation.ID)
			idleInput := visitorInput(channelID, "")
			idle := f.receive(t, &idleInput, "已回答的问题")
			runQueuedAgentRun(t, f.db, agentrunaction.NewExecuteAction(f.db, f.tasks, &testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
				claimed, err := feed.Claim(ctx, 1)
				return agentruntime.RunResult{Content: "已回答", EndSeq: claimed.EndSeq}, err
			}}, testAttachmentReader(f.db), nil), idle.Conversation.ID)

			handoff := testServiceSessionHandoff(f.db)
			if change == "停用" {
				if _, err := agentaction.NewUpdateStatusAction(f.db, handoff).Execute(ctx, f.identity, agent.ID, domain.UserStatusInactive); err != nil {
					t.Fatal(err)
				}
			} else {
				memberRole := servermodels.Role{}
				if err := f.db.NewSelect().Model(&memberRole).Where("organization_id = ? AND kind = ?", f.identity.Organization.ID, domain.RoleKindMember).Scan(ctx); err != nil {
					t.Fatal(err)
				}
				if _, err := agentaction.NewUpdateAgentAction(f.db, handoff).Execute(ctx, f.identity, agent.ID, agentaction.UpdateInput{
					DisplayName: agent.DisplayName, RoleID: memberRole.ID, TeamIDs: []string{}, WorkStatus: domain.WorkStatusWorking,
				}); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.db.NewSelect().Model(&pendingRun).WherePK().Scan(ctx); err != nil || pendingRun.Status != string(domain.AgentRunStatusCancelled) ||
				pendingRun.ErrorCode == nil || *pendingRun.ErrorCode != string(domain.AgentRunErrorCodeAgentUnavailable) {
				t.Fatalf("pending run = %+v, error = %v", pendingRun, err)
			}
			for _, conversationID := range []string{pending.Conversation.ID, idle.Conversation.ID} {
				events := handoffEvents(t, f.db, conversationID)
				if len(events) != 1 || events[0].Reason != domain.AgentHandoffReasonAgentUnavailable || events[0].AgentRunID != nil {
					t.Fatalf("events = %+v", events)
				}
				session := loadSession(t, f.db, events[0].ServiceSessionID)
				if session.AssigneeIdentityID != nil {
					t.Fatalf("session = %+v", session)
				}
				notices, err := f.db.NewSelect().Model((*servermodels.Message)(nil)).
					Where("msg.conversation_id = ? AND msg.idempotency_key LIKE ?", conversationID, "handoff:"+session.ID+":%").
					Where("msg.type = ?", domain.MessageTypeText).Count(ctx)
				if err != nil || notices != 1 {
					t.Fatalf("handoff notices = %d, error = %v", notices, err)
				}
			}
			lane := servermodels.AgentLane{}
			if err := f.db.NewSelect().Model(&lane).Where("al.id = ?", pendingRun.LaneID).Scan(ctx); err != nil || lane.ProcessedSeq != lane.DesiredSeq {
				t.Fatalf("lane = %+v, error = %v", lane, err)
			}
		})
	}
}

// testInboundRoutingVersusEligibility 验证新访客入站与停用、改角色并发交错后，没有开放周期留在失去资格的 AI 员工名下。
func testInboundRoutingVersusEligibility(t *testing.T, f handoffFixture) {
	ctx := context.Background()
	memberRole := servermodels.Role{}
	if err := f.db.NewSelect().Model(&memberRole).Where("organization_id = ? AND kind = ?", f.identity.Organization.ID, domain.RoleKindMember).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"停用", "改角色"} {
		t.Run(change, func(t *testing.T) {
			agent := f.newAgent(t, change+"并发客服")
			channelID := f.newChannel(t, agent.IdentityID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
			var wait sync.WaitGroup
			errs := make(chan error, 9)
			for index := range 8 {
				wait.Add(1)
				go func() {
					defer wait.Done()
					input := visitorInput(channelID, "并发入站")
					_, err := conversationaction.NewReceiveWebsiteCustomerMessageAction(f.db, agentrunaction.NewScheduler(f.tasks)).Execute(ctx, input)
					errs <- err
				}()
				if index == 3 {
					wait.Add(1)
					go func() {
						defer wait.Done()
						var err error
						if change == "停用" {
							_, err = agentaction.NewUpdateStatusAction(f.db, testServiceSessionHandoff(f.db)).Execute(ctx, f.identity, agent.ID, domain.UserStatusInactive)
						} else {
							_, err = agentaction.NewUpdateAgentAction(f.db, testServiceSessionHandoff(f.db)).Execute(ctx, f.identity, agent.ID, agentaction.UpdateInput{
								DisplayName: agent.DisplayName, RoleID: memberRole.ID, TeamIDs: []string{}, WorkStatus: domain.WorkStatusWorking,
							})
						}
						errs <- err
					}()
				}
			}
			wait.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			stranded, err := f.db.NewSelect().Model((*servermodels.ServiceSession)(nil)).
				Where("ss.assignee_identity_id = ? AND ss.status = ?", agent.IdentityID, domain.ServiceSessionStatusOpen).Count(ctx)
			if err != nil || stranded != 0 {
				t.Fatalf("open sessions left on unavailable agent = %d, error = %v", stranded, err)
			}
		})
	}
}

// testInboundUnavailableAssignee 验证负责人在管理操作之外失去接客资格时，下一条客户消息在入站事务内触发交接。
func testInboundUnavailableAssignee(t *testing.T, f handoffFixture) {
	ctx := context.Background()
	agent := f.newAgent(t, "失效负责人客服")
	channelID := f.newChannel(t, agent.IdentityID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	input := visitorInput(channelID, "")
	first := f.receive(t, &input, "第一条")
	run := f.queuedRun(t, first.Conversation.ID)
	if _, err := f.db.NewUpdate().Model((*servermodels.Agent)(nil)).Set("status = ?", domain.UserStatusInactive).Where("id = ?", agent.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	f.receive(t, &input, "第二条")
	events := handoffEvents(t, f.db, first.Conversation.ID)
	if len(events) != 1 || events[0].Reason != domain.AgentHandoffReasonAgentUnavailable {
		t.Fatalf("events = %+v", events)
	}
	if session := loadSession(t, f.db, run.ScopeID); session.AssigneeIdentityID != nil {
		t.Fatalf("session = %+v", session)
	}
	if err := f.db.NewSelect().Model(&run).WherePK().Scan(ctx); err != nil || run.Status != string(domain.AgentRunStatusCancelled) {
		t.Fatalf("run = %+v, error = %v", run, err)
	}
}

// testTelegramModelHandoff 验证 Telegram 会话主动转人工只产生一次对客投递，重复执行不追加。
func testTelegramModelHandoff(t *testing.T, f handoffFixture) {
	ctx := context.Background()
	fixture := newAgentTelegramFixture(t, f.db, f.identity, f.roleID, f.providerID, f.modelID)
	executor := agentrunaction.NewExecuteAction(f.db, fixture.tasks, handoffRuntime("为您转接人工", "需要人工确认", nil), testAttachmentReader(f.db), nil)
	for range 2 {
		if err := executor.Execute(ctx, agentrunaction.RunInput{RunID: fixture.run.ID}); err != nil {
			t.Fatal(err)
		}
	}
	fixture.reload(t)
	var deliveries []servermodels.CustomerMessageDelivery
	if err := f.db.NewSelect().Model(&deliveries).Where("cmd.conversation_id = ?", fixture.run.ConversationID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if fixture.run.ResponseMessageID == nil || len(deliveries) != 1 || deliveries[0].MessageID != *fixture.run.ResponseMessageID ||
		len(handoffEvents(t, f.db, fixture.run.ConversationID)) != 1 {
		t.Fatalf("run = %+v, deliveries = %+v", fixture.run, deliveries)
	}
}

// testChannelEditVersusDeactivation 验证停用 AI 员工持有身份锁时，以其为路由目标的渠道编辑先等待身份锁再锁渠道，两者不形成循环等待。
func testChannelEditVersusDeactivation(t *testing.T, f handoffFixture) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	agent := f.newAgent(t, "渠道编辑并发客服")
	channelID := f.newChannel(t, agent.IdentityID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	// 渠道编辑由另一名成员发起，两个操作人各自持有自己的账号锁。
	email := "channel-editor-" + uuid.NewV7().String()[:8] + "@handoff.test"
	if _, err := useraction.NewCreateUserAction(f.db).Execute(ctx, f.identity, useraction.CreateInput{
		DisplayName: "渠道编辑成员", Email: email, Password: "password123", RoleID: f.identity.OrganizationIdentity.RoleID,
	}); err != nil {
		t.Fatal(err)
	}
	editor, err := authaction.NewLoginAction(f.db).Execute(ctx, authaction.LoginInput{OrganizationID: f.identity.Organization.ID, Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	gated := bun.NewDB(f.db.DB, f.db.Dialect())
	gated.AddQueryHook(chatQueryHook{})
	// 停用事务取得 AI 员工身份排他锁后暂停。
	gate := newChatQueryGate(t, false, 1, func(event *bun.QueryEvent) bool {
		return strings.Contains(event.Query, "organization_identities") && strings.Contains(event.Query, "FOR UPDATE OF oi")
	})
	deactivated, edited := make(chan error, 1), make(chan error, 1)
	go func() {
		_, err := agentaction.NewUpdateStatusAction(gated, testServiceSessionHandoff(f.db)).Execute(context.WithValue(ctx, chatQueryGateKey{}, gate), f.identity, agent.ID, domain.UserStatusInactive)
		deactivated <- err
	}()
	waitChatSignal(t, ctx, gate.reached)
	go func() {
		_, err := channelaction.NewUpdateMessageChannelAction(f.db).Execute(ctx, editor.Identity, channelID, channelaction.MessageChannelInput{
			Name: "渠道编辑并发", DefaultLocale: domain.LocaleChineseSimplified,
			NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeMember, ID: agent.IdentityID},
			FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		})
		edited <- err
	}()
	waitChatDatabaseLock(t, ctx, f.db, "organization_identities", agent.IdentityID)
	gate.open()
	if err := waitChatResult(t, ctx, deactivated); err != nil {
		t.Fatalf("deactivate agent: %v", err)
	}
	// 渠道编辑在停用提交后校验目标，AI 员工已不可用时按校验失败返回。
	var validation *channelaction.ValidationError
	if err := waitChatResult(t, ctx, edited); err != nil && !errors.As(err, &validation) {
		t.Fatalf("edit channel: %v", err)
	}
	channel := servermodels.Channel{}
	if err := f.db.NewSelect().Model(&channel).Where("c.id = ?", channelID).Scan(ctx); err != nil ||
		channel.InitialRoutingTargetType != string(domain.ChannelRoutingTargetTypePublicQueue) {
		t.Fatalf("channel = %+v, error = %v", channel, err)
	}
}

// testTelegramInboundHandoffVersusRunFailure 验证已持有会话锁的入站事务发现负责人失效时，交接只读取外发目标，与先锁渠道身份的运行收尾不形成循环等待。
func testTelegramInboundHandoffVersusRunFailure(t *testing.T, f handoffFixture) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	fixture := newAgentTelegramFixture(t, f.db, f.identity, f.roleID, f.providerID, f.modelID)
	if _, err := f.db.NewUpdate().Model((*servermodels.Agent)(nil)).Set("status = ?", domain.UserStatusInactive).
		Where("identity_id = ?", fixture.run.AgentIdentityID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	var messageID string
	if err := f.db.NewSelect().Model((*servermodels.Message)(nil)).Column("id").
		Where("msg.conversation_id = ? AND msg.type = ?", fixture.run.ConversationID, domain.MessageTypeText).
		OrderExpr("msg.message_seq DESC").Limit(1).Scan(ctx, &messageID); err != nil {
		t.Fatal(err)
	}
	gated := bun.NewDB(f.db.DB, f.db.Dialect())
	gated.AddQueryHook(chatQueryHook{})
	// 运行失败收尾取得渠道身份锁后、申请会话锁前暂停。
	gate := newChatQueryGate(t, false, 1, func(event *bun.QueryEvent) bool {
		return strings.Contains(event.Query, "contact_channel_identities") && strings.Contains(event.Query, "FOR UPDATE")
	})
	failed, scheduled := make(chan error, 1), make(chan error, 1)
	go func() {
		executor := agentrunaction.NewExecuteAction(gated, fixture.tasks, nil, testAttachmentReader(f.db), nil)
		failed <- executor.FinalizeFailure(context.WithValue(ctx, chatQueryGateKey{}, gate), agentrunaction.RunInput{RunID: fixture.run.ID}, errors.New("运行失败"))
	}()
	waitChatSignal(t, ctx, gate.reached)
	go func() {
		scheduled <- realtime.RunInTx(ctx, f.db, func(ctx context.Context, tx bun.Tx) error {
			_, session, err := chatstate.LockCustomerServiceSession(ctx, tx, fixture.run.OrganizationID, fixture.run.ConversationID)
			if err != nil {
				return err
			}
			_, err = agentrunaction.NewScheduler(fixture.tasks).ScheduleCustomerAuto(ctx, tx, fixture.run.OrganizationID, fixture.run.ConversationID, session.ID, messageID)
			return err
		})
	}()
	// 入站事务在运行收尾暂停期间独立完成交接。
	select {
	case err := <-scheduled:
		if err != nil {
			t.Fatalf("inbound handoff: %v", err)
		}
	case <-time.After(5 * time.Second):
		gate.open()
		t.Fatal("inbound handoff waited for the channel identity lock held by run failure")
	}
	gate.open()
	if err := waitChatResult(t, ctx, failed); err != nil {
		t.Fatalf("finalize run failure: %v", err)
	}
	fixture.reload(t)
	var deliveries []servermodels.CustomerMessageDelivery
	if err := f.db.NewSelect().Model(&deliveries).Where("cmd.conversation_id = ?", fixture.run.ConversationID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	events := handoffEvents(t, f.db, fixture.run.ConversationID)
	if fixture.run.Status != string(domain.AgentRunStatusCancelled) || len(events) != 1 ||
		events[0].Reason != domain.AgentHandoffReasonAgentUnavailable || len(deliveries) != 1 {
		t.Fatalf("run = %+v, events = %+v, deliveries = %+v", fixture.run, events, deliveries)
	}
}

// testServiceSessionOperationEvents 验证领取、接管、转交、关闭与重开各写一条仅成员可见的周期事件，且不改变会话摘要与活动时间。
func testServiceSessionOperationEvents(t *testing.T, f handoffFixture) {
	ctx := context.Background()
	agent := f.newAgent(t, "周期事件客服")
	channel, err := channelaction.NewCreateMessageChannelAction(f.db).Execute(ctx, f.identity, channelaction.CreateMessageChannelInput{
		Type: domain.ChannelTypeWebsite, Name: "周期事件验证", DefaultLocale: domain.LocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
	})
	if err != nil {
		t.Fatal(err)
	}
	email := "session-events-" + uuid.NewV7().String()[:8] + "@handoff.test"
	if _, err := useraction.NewCreateUserAction(f.db).Execute(ctx, f.identity, useraction.CreateInput{
		DisplayName: "接管成员", Email: email, Password: "password123", RoleID: f.roleID,
	}); err != nil {
		t.Fatal(err)
	}
	other, err := authaction.NewLoginAction(f.db).Execute(ctx, authaction.LoginInput{OrganizationID: f.identity.Organization.ID, Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	input := visitorInput(channel.ID, "")
	first := f.receive(t, &input, "有人吗")
	conversationID := first.Conversation.ID
	coordinator := testServiceSessionHandoff(f.db)
	scheduler := agentrunaction.NewScheduler(f.tasks)
	owner, member := f.identity.OrganizationIdentity.ID, other.Identity.OrganizationIdentity.ID
	// 成员回复无人负责的周期即领取。
	reply, err := conversationaction.NewSendCustomerTextMessageAction(f.db, nil).Execute(ctx, f.identity, conversationaction.CustomerTextMessageInput{
		ConversationID: conversationID, ClientMessageID: uuid.NewV7().String(), Body: "在的",
	})
	if err != nil {
		t.Fatal(err)
	}
	summary := servermodels.Conversation{}
	if err := f.db.NewSelect().Model(&summary).Where("cv.id = ?", conversationID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewClaimServiceSessionAction(f.db, coordinator).Execute(ctx, other.Identity, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewTransferServiceSessionAction(f.db, coordinator, scheduler).Execute(ctx, other.Identity, conversationaction.TransferServiceSessionInput{
		ConversationID: conversationID, AssigneeIdentityID: agent.IdentityID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewClaimServiceSessionAction(f.db, coordinator).Execute(ctx, f.identity, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewCloseServiceSessionAction(f.db, coordinator).Execute(ctx, f.identity, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewReopenServiceSessionAction(f.db).Execute(ctx, f.identity, conversationID); err != nil {
		t.Fatal(err)
	}

	var messages []servermodels.Message
	if err := f.db.NewSelect().Model(&messages).
		Where("msg.conversation_id = ? AND msg.type = ?", conversationID, domain.MessageTypeSystem).
		OrderExpr("msg.message_seq").Scan(ctx); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		eventType domain.ConversationSystemEventType
		actor     string
		from      *string
		target    *string
	}{
		{domain.ConversationSystemEventServiceSessionClaimed, owner, nil, nil},
		{domain.ConversationSystemEventServiceSessionTakenOver, member, &owner, nil},
		{domain.ConversationSystemEventServiceSessionTransferred, member, &member, &agent.IdentityID},
		{domain.ConversationSystemEventServiceSessionTakenOver, owner, &agent.IdentityID, nil},
		{domain.ConversationSystemEventServiceSessionClosed, owner, nil, nil},
		{domain.ConversationSystemEventServiceSessionReopened, owner, nil, nil},
	}
	if len(messages) != len(want) {
		t.Fatalf("service session events = %d, want %d", len(messages), len(want))
	}
	for index, message := range messages {
		event := domain.ServiceSessionOperatedEvent{}
		if err := json.Unmarshal(message.SystemEventPayload, &event); err != nil {
			t.Fatal(err)
		}
		expected := want[index]
		if message.SystemEventType == nil || *message.SystemEventType != string(expected.eventType) ||
			message.Visibility != string(domain.MessageVisibilityInternalOnly) || message.ServiceSessionID == nil ||
			event.ActorIdentityID != expected.actor || event.ActorDisplayName == "" ||
			(expected.from == nil) != (event.FromIdentityID == nil) || (expected.from != nil && (*event.FromIdentityID != *expected.from || event.FromDisplayName == nil)) ||
			(expected.target == nil) != (event.Target == nil) || (expected.target != nil && (event.Target.IdentityID == nil || *event.Target.IdentityID != *expected.target)) {
			t.Fatalf("event %d = %s %+v", index, *message.SystemEventType, event)
		}
	}
	// 周期事件不改变会话摘要与活动时间。
	after := servermodels.Conversation{}
	if err := f.db.NewSelect().Model(&after).Where("cv.id = ?", conversationID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if after.LastMessageID == nil || *after.LastMessageID != reply.ID || after.LastActivityAt == nil || summary.LastActivityAt == nil || !after.LastActivityAt.Equal(*summary.LastActivityAt) {
		t.Fatalf("conversation summary before = %+v, after = %+v", summary, after)
	}
	// 成员历史按群聊事件的 actor 结构返回操作人。
	history, err := conversationaction.NewListConversationMessagesQuery(f.db).Execute(ctx, f.identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID})
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range history.Messages {
		if message.SystemEvent != nil && (message.SystemEvent.ActorIdentityID == nil || message.SystemEvent.ServiceSessionID == nil) {
			t.Fatalf("history event = %+v", message.SystemEvent)
		}
	}
}
