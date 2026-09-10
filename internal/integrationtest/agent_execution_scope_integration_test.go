//go:build server

package integrationtest

import (
	"context"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	aiprovideraction "github.com/runforyou-ai/cervi/internal/actions/aiprovider"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
)

type executionScopeFixture struct {
	customerReadFixture
	agentIdentityID string
	scheduler       *agentrunaction.Scheduler
	coordinator     *agentrunaction.ExecuteAction
	transfer        *conversationaction.TransferServiceSessionAction
	claim           *conversationaction.ClaimServiceSessionAction
	close           *conversationaction.CloseServiceSessionAction
}

// newExecutionScopeFixture 建立可转交给 AI 客服的网站客户会话。
func newExecutionScopeFixture(t *testing.T) executionScopeFixture {
	t.Helper()
	ctx := context.Background()
	f := newCustomerReadFixture(t)
	provider, err := aiprovideraction.NewCreateAIProviderAction(f.db).Execute(ctx, f.owner, aiprovideraction.Input{
		Brand: domain.AIProviderBrandOpenAI, Name: uuid.NewV7().String(), APIKey: "test-key", APIURL: "https://models.test/v1",
		Models: []aiprovideraction.Model{{
			Identifier: "chat-a", Name: "对话 A", Type: domain.AIModelTypeChat,
			InputModalities: []domain.AIModelInputModality{domain.AIModelInputModalityText}, ContextWindow: 8192, MaxOutputTokens: 4096,
		}},
	})
	if err != nil {
		t.Fatalf("创建模型服务失败：%v", err)
	}
	agent, err := agentaction.NewCreateAgentAction(f.db).Execute(ctx, f.owner, agentaction.CreateInput{
		DisplayName: "执行范围助手", RoleID: f.owner.OrganizationIdentity.RoleID,
		Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{
			ProviderID: provider.ID, ModelIdentifier: "chat-a", SystemInstruction: "回答客户问题",
		}},
	})
	if err != nil {
		t.Fatalf("创建 AI 员工失败：%v", err)
	}
	tasks := servertask.New(f.db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatalf("注册任务失败：%v", err)
	}
	scheduler := agentrunaction.NewScheduler(tasks)
	coordinator := agentrunaction.NewExecuteAction(f.db, tasks, nil)
	return executionScopeFixture{
		customerReadFixture: f, agentIdentityID: agent.IdentityID, scheduler: scheduler, coordinator: coordinator,
		transfer: conversationaction.NewTransferServiceSessionAction(f.db, coordinator, scheduler),
		claim:    conversationaction.NewClaimServiceSessionAction(f.db, coordinator),
		close:    conversationaction.NewCloseServiceSessionAction(f.db, coordinator),
	}
}

// transferToAgent 把当前处理周期交给 AI 客服并返回周期编号。
func (f executionScopeFixture) transferToAgent(t *testing.T, ctx context.Context) string {
	t.Helper()
	if _, err := f.claim.Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatalf("领取周期失败：%v", err)
	}
	session, err := f.transfer.Execute(ctx, f.owner, conversationaction.TransferServiceSessionInput{
		ConversationID: f.conversationID, AssigneeIdentityID: f.agentIdentityID,
	})
	if err != nil {
		t.Fatalf("转交给 AI 客服失败：%#v", err)
	}
	return session.ID
}

// activeRunCount 统计会话内尚未终结的 Agent 运行。
func (f executionScopeFixture) activeRunCount(t *testing.T, ctx context.Context) int {
	t.Helper()
	count, err := f.db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		Where("agr.conversation_id = ?", f.conversationID).
		Where("agr.status IN (?, ?)", domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return count
}

// laneForSession 读取指定客服周期的输入队列。
func (f executionScopeFixture) laneForSession(t *testing.T, ctx context.Context, sessionID string) servermodels.AgentLane {
	t.Helper()
	lane := servermodels.AgentLane{}
	if err := f.db.NewSelect().Model(&lane).
		Where("al.scope_kind = ? AND al.scope_id = ?", domain.AgentExecutionScopeServiceSession, sessionID).
		Where("al.agent_identity_id = ?", f.agentIdentityID).
		Scan(ctx); err != nil {
		t.Fatal(err)
	}
	return lane
}

// TestAgentExecutionScopeUniqueness 验证同一执行范围内只允许一个活动运行。
func TestAgentExecutionScopeUniqueness(t *testing.T) {
	f := newExecutionScopeFixture(t)
	ctx := context.Background()
	sessionID := f.transferToAgent(t, ctx)
	if count := f.activeRunCount(t, ctx); count != 1 {
		t.Fatalf("转交后活动运行 = %d，期望 1", count)
	}
	existing := servermodels.AgentRun{}
	if err := f.db.NewSelect().Model(&existing).
		Where("agr.conversation_id = ?", f.conversationID).
		Where("agr.status = ?", domain.AgentRunStatusQueued).
		Scan(ctx); err != nil {
		t.Fatal(err)
	}
	duplicate := &servermodels.AgentRun{
		ID: uuid.NewV7().String(), OrganizationID: existing.OrganizationID, ConversationID: existing.ConversationID,
		AgentIdentityID: existing.AgentIdentityID, AgentRevisionID: existing.AgentRevisionID, LaneID: existing.LaneID,
		ScopeKind: string(domain.AgentExecutionScopeServiceSession), ScopeID: sessionID,
		Status: string(domain.AgentRunStatusQueued), InputStartSeq: 1,
	}
	_, err := f.db.NewInsert().Model(duplicate).
		Column("id", "organization_id", "conversation_id", "agent_identity_id", "agent_revision_id", "lane_id", "scope_kind", "scope_id", "status", "input_start_seq").
		Exec(ctx)
	if !pgerr.UniqueViolationOn(err, "agent_runs_active_scope_unique") {
		t.Fatalf("同范围第二个活动运行未被唯一索引拒绝：%v", err)
	}
}

// TestAgentExecutionScopeSessionBoundary 验证客服周期切换后输入队列按新周期重新编号。
func TestAgentExecutionScopeSessionBoundary(t *testing.T) {
	f := newExecutionScopeFixture(t)
	ctx := context.Background()
	firstSession := f.transferToAgent(t, ctx)
	if _, err := f.visitorMessage(ctx, "第一周期追问"); err != nil {
		t.Fatal(err)
	}
	firstLane := f.laneForSession(t, ctx, firstSession)
	if firstLane.DesiredSeq != 2 {
		t.Fatalf("首个周期队列水位 = %+v，期望 desired_seq 为 2", firstLane)
	}
	if count := f.activeRunCount(t, ctx); count != 1 {
		t.Fatalf("运行期间新增输入后活动运行 = %d，期望仍为 1", count)
	}
	if _, err := f.claim.Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.close.Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatal(err)
	}
	settled := f.laneForSession(t, ctx, firstSession)
	if settled.ProcessedSeq != settled.DesiredSeq {
		t.Fatalf("关闭后首个周期队列未结算：%+v", settled)
	}
	reopened, err := f.visitorMessage(ctx, "新周期首问")
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Conversation.ServiceSessionID == firstSession {
		t.Fatalf("期望新的客服周期，实际仍为 %s", firstSession)
	}
	secondSession := f.transferToAgent(t, ctx)
	secondLane := f.laneForSession(t, ctx, secondSession)
	if secondLane.ID == firstLane.ID || secondLane.DesiredSeq != 1 || secondLane.ProcessedSeq != 0 {
		t.Fatalf("新周期队列 = %+v，期望独立队列且序号从 1 开始", secondLane)
	}
	var secondInputSeqs []int64
	if err := f.db.NewSelect().Model((*servermodels.AgentInput)(nil)).
		ColumnExpr("ai.input_seq").
		Where("ai.lane_id = ?", secondLane.ID).
		OrderExpr("ai.input_seq ASC").
		Scan(ctx, &secondInputSeqs); err != nil {
		t.Fatal(err)
	}
	if len(secondInputSeqs) != 1 || secondInputSeqs[0] != 1 {
		t.Fatalf("新周期输入序号 = %v，期望仅有序号 1", secondInputSeqs)
	}
	reread := f.laneForSession(t, ctx, firstSession)
	if reread.ID != firstLane.ID || reread.DesiredSeq != settled.DesiredSeq || reread.ProcessedSeq != settled.ProcessedSeq {
		t.Fatalf("新周期建立后首个周期队列被改写：%+v", reread)
	}
	oldInputCount, err := f.db.NewSelect().Model((*servermodels.AgentInput)(nil)).
		Where("ai.lane_id = ?", firstLane.ID).Count(ctx)
	if err != nil || int64(oldInputCount) != settled.DesiredSeq {
		t.Fatalf("旧周期输入数量 = %d，期望 %d，error = %v", oldInputCount, settled.DesiredSeq, err)
	}
}

// TestAgentExecutionScopeHandoverKeepsSingleRun 验证转交、领取与关闭不产生重叠运行。
func TestAgentExecutionScopeHandoverKeepsSingleRun(t *testing.T) {
	f := newExecutionScopeFixture(t)
	ctx := context.Background()
	f.transferToAgent(t, ctx)
	if count := f.activeRunCount(t, ctx); count != 1 {
		t.Fatalf("转交后活动运行 = %d，期望 1", count)
	}
	for _, step := range []struct {
		name       string
		operate    func() error
		wantActive int
	}{
		{"领取", func() error { _, err := f.claim.Execute(ctx, f.owner, f.conversationID); return err }, 0},
		{"再次转交", func() error { f.transferToAgent(t, ctx); return nil }, 1},
		{"收回", func() error { _, err := f.claim.Execute(ctx, f.owner, f.conversationID); return err }, 0},
		{"关闭", func() error { _, err := f.close.Execute(ctx, f.owner, f.conversationID); return err }, 0},
	} {
		if err := step.operate(); err != nil {
			t.Fatalf("%s 失败：%v", step.name, err)
		}
		if count := f.activeRunCount(t, ctx); count != step.wantActive {
			t.Fatalf("%s 后活动运行 = %d，期望 %d", step.name, count, step.wantActive)
		}
	}
	// 真人领取取消原负责人运行，取消原因固定为负责人变化。
	cancelled := make([]string, 0)
	if err := f.db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		ColumnExpr("COALESCE(agr.error_code, '')").
		Where("agr.conversation_id = ?", f.conversationID).
		Where("agr.status = ?", domain.AgentRunStatusCancelled).
		Scan(ctx, &cancelled); err != nil {
		t.Fatal(err)
	}
	if len(cancelled) != 2 {
		t.Fatalf("取消运行数量 = %d，期望 2", len(cancelled))
	}
	for _, code := range cancelled {
		if code != string(domain.AgentRunErrorCodeAssigneeChanged) {
			t.Fatalf("取消原因 = %q，期望 %q", code, domain.AgentRunErrorCodeAssigneeChanged)
		}
	}
}
