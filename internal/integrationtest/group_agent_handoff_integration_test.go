//go:build server

package integrationtest

import (
	"context"
	"testing"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// submitReply 以结构化结果结束一次群内运行，并返回执行的 AI 员工身份。
func (f groupAgentFixture) submitReply(t *testing.T, result agentruntime.RunResult, inspect func(agentruntime.RunRequest)) string {
	t.Helper()
	ctx := context.Background()
	run := f.activeRun(t)
	if run == nil {
		t.Fatal("群内没有待执行的运行")
	}
	runtime := testAgentRuntime{run: func(ctx context.Context, request agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		triggers, err := feed.Peek(ctx, 0)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		claimed, err := feed.Claim(ctx, triggers[len(triggers)-1].Seq)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		if inspect != nil {
			inspect(request)
		}
		result.EndSeq = claimed.EndSeq
		return result, nil
	}}
	if err := agentrunaction.NewExecuteAction(f.db, f.tasks, runtime).Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	return run.AgentIdentityID
}

// latestInput 读取指定队列最新一条输入。
func (f groupAgentFixture) latestInput(t *testing.T, laneID string) *servermodels.AgentInput {
	t.Helper()
	input := &servermodels.AgentInput{}
	if err := f.db.NewSelect().Model(input).
		Where("ai.lane_id = ?", laneID).
		OrderExpr("ai.input_seq DESC").Limit(1).Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	return input
}

// agentByIdentity 按身份编号返回测试 AI 员工。
func (f groupAgentFixture) agentByIdentity(t *testing.T, identityID string) *agentaction.Agent {
	t.Helper()
	for _, agent := range f.agents {
		if agent.IdentityID == identityID {
			return agent
		}
	}
	t.Fatalf("未找到身份为 %s 的 AI 员工", identityID)
	return nil
}

// testGroupAgentHandoff 验证结构化结果、接力、静默与接力深度上限。
func testGroupAgentHandoff(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string) {
	t.Helper()
	ctx := context.Background()
	agents := newGroupAgentCollaborators(t, db, identity, roleID, providerID, modelID)

	t.Run("结束工具限定可点名成员", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, agents)
		f.post(t, "请看一下", []string{f.agents[0].IdentityID}, "")
		f.submitReply(t, agentruntime.RunResult{Outcome: agentruntime.RunOutcomeReply, Content: "看过了"}, func(request agentruntime.RunRequest) {
			if request.GroupReply == nil {
				t.Fatal("群内运行应使用结构化结束工具")
			}
			candidates := request.GroupReply.MentionCandidates
			if len(candidates) != 2 {
				t.Fatalf("可点名成员 = %v，期望群主与另一位 AI 员工", candidates)
			}
			for _, name := range candidates {
				if name == f.agents[0].DisplayName {
					t.Fatalf("可点名成员不应包含自己：%v", candidates)
				}
			}
		})
	})

	t.Run("点名另一位 AI 员工形成接力", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, agents)
		f.post(t, "请第一位看一下", []string{f.agents[0].IdentityID}, "")
		first := f.submitReply(t, agentruntime.RunResult{
			Outcome: agentruntime.RunOutcomeReply, Content: "成本我算过，请确认排期",
			Mentions: []string{f.agents[1].DisplayName},
		}, nil)
		next := f.activeRun(t)
		if next == nil || next.AgentIdentityID == first {
			t.Fatalf("接力未安排到被点名的 AI 员工：%+v", next)
		}
		input := &servermodels.AgentInput{}
		if err := db.NewSelect().Model(input).
			Where("ai.lane_id = ?", next.LaneID).
			OrderExpr("ai.input_seq DESC").Limit(1).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if input.Kind != string(domain.AgentInputKindHandoff) || input.Depth != 1 {
			t.Fatalf("接力输入 = %+v，期望 kind 为 handoff 且 depth 为 1", input)
		}
		// 接力挂在发起方的公开回复上，被点名方据此看到确认内容。
		var sourceBody string
		if err := db.NewSelect().Model((*servermodels.Message)(nil)).
			ColumnExpr("msg.body").Where("msg.id = ?", input.SourceMessageID).Scan(ctx, &sourceBody); err != nil {
			t.Fatal(err)
		}
		if sourceBody != "成本我算过，请确认排期" {
			t.Fatalf("接力来源消息 = %q", sourceBody)
		}
		mentionCount, err := db.NewSelect().Model((*servermodels.MessageMention)(nil)).
			Where("mm.message_id = ?", input.SourceMessageID).Count(ctx)
		if err != nil || mentionCount != 1 {
			t.Fatalf("接力回复的提醒关系 = %d，期望 1，error = %v", mentionCount, err)
		}
	})

	t.Run("真人消息重置接力深度", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, agents)
		f.post(t, "请第一位看一下", []string{f.agents[0].IdentityID}, "")
		f.submitReply(t, agentruntime.RunResult{
			Outcome: agentruntime.RunOutcomeReply, Content: "请第二位确认",
			Mentions: []string{f.agents[1].DisplayName},
		}, nil)
		second := f.agentByIdentity(t, f.activeRun(t).AgentIdentityID)
		f.submitReply(t, agentruntime.RunResult{Outcome: agentruntime.RunOutcomeReply, Content: "确认完毕"}, nil)
		f.post(t, "再看一次", []string{second.IdentityID}, "")
		if input := f.latestInput(t, f.activeRun(t).LaneID); input.Depth != 0 || input.Kind != string(domain.AgentInputKindMention) {
			t.Fatalf("真人点名输入 = %+v，期望 depth 为 0 且 kind 为 mention", input)
		}
		// 真人输入之后的接力从深度一重新开始。
		other := f.agents[0].DisplayName
		if second.IdentityID == f.agents[0].IdentityID {
			other = f.agents[1].DisplayName
		}
		f.submitReply(t, agentruntime.RunResult{
			Outcome: agentruntime.RunOutcomeReply, Content: "再请对方确认", Mentions: []string{other},
		}, nil)
		if input := f.latestInput(t, f.activeRun(t).LaneID); input.Depth != 1 || input.Kind != string(domain.AgentInputKindHandoff) {
			t.Fatalf("重置后的接力输入 = %+v，期望 depth 为 1", input)
		}
	})

	t.Run("点名真人只写提醒不安排执行", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, agents)
		f.post(t, "请第一位看一下", []string{f.agents[0].IdentityID}, "")
		f.submitReply(t, agentruntime.RunResult{
			Outcome: agentruntime.RunOutcomeReply, Content: "请群主确认", Mentions: []string{identity.OrganizationIdentity.DisplayName},
		}, nil)
		if run := f.activeRun(t); run != nil {
			t.Fatalf("点名真人不应安排执行：%+v", run)
		}
		count, err := db.NewSelect().Model((*servermodels.MessageMention)(nil)).
			Join("JOIN messages AS msg ON msg.id = mm.message_id").
			Where("msg.conversation_id = ? AND msg.body = ?", f.groupID, "请群主确认").Count(ctx)
		if err != nil || count != 1 {
			t.Fatalf("点名真人的提醒关系 = %d，期望 1，error = %v", count, err)
		}
	})

	t.Run("无法解析的点名被忽略", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, agents)
		f.post(t, "请第一位看一下", []string{f.agents[0].IdentityID}, "")
		f.submitReply(t, agentruntime.RunResult{
			Outcome: agentruntime.RunOutcomeReply, Content: "点名不存在的成员", Mentions: []string{"查无此人"},
		}, nil)
		if run := f.activeRun(t); run != nil {
			t.Fatalf("无效点名不应安排执行：%+v", run)
		}
		count, err := db.NewSelect().Model((*servermodels.MessageMention)(nil)).
			Join("JOIN messages AS msg ON msg.id = mm.message_id").
			Where("msg.conversation_id = ? AND msg.body = ?", f.groupID, "点名不存在的成员").Count(ctx)
		if err != nil || count != 0 {
			t.Fatalf("无效点名的提醒关系 = %d，期望 0，error = %v", count, err)
		}
	})

	t.Run("超出深度上限的接力被拒绝", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, agents)
		f.post(t, "请第一位看一下", []string{f.agents[0].IdentityID}, "")
		// 连续接力直到达到上限，之后的点名不再产生输入。
		for range 4 {
			run := f.activeRun(t)
			if run == nil {
				break
			}
			target := f.agents[0].DisplayName
			if run.AgentIdentityID == f.agents[0].IdentityID {
				target = f.agents[1].DisplayName
			}
			f.submitReply(t, agentruntime.RunResult{
				Outcome: agentruntime.RunOutcomeReply, Content: "继续确认", Mentions: []string{target},
			}, nil)
		}
		if run := f.activeRun(t); run != nil {
			t.Fatalf("超出接力深度上限后仍在调度：%+v", run)
		}
		// 被拒绝的只是接力执行，公开回复与提醒关系照常保留。
		lastMentions, err := db.NewSelect().Model((*servermodels.MessageMention)(nil)).
			Join("JOIN messages AS msg ON msg.id = mm.message_id").
			Where("msg.conversation_id = ? AND msg.body = ?", f.groupID, "继续确认").Count(ctx)
		if err != nil || lastMentions != 4 {
			t.Fatalf("接力回复的提醒关系 = %d，期望 4，error = %v", lastMentions, err)
		}
		maxDepth := 0
		if err := db.NewSelect().TableExpr("agent_inputs AS ai").
			ColumnExpr("COALESCE(MAX(ai.depth), 0)").
			Join("JOIN agent_lanes AS al ON al.id = ai.lane_id").
			Where("al.conversation_id = ?", f.groupID).Scan(ctx, &maxDepth); err != nil {
			t.Fatal(err)
		}
		if maxDepth != 3 {
			t.Fatalf("最大接力深度 = %d，期望 3", maxDepth)
		}
	})

	t.Run("静默结果不写消息但推进轮转", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, agents)
		f.post(t, "两位看下", []string{f.agents[0].IdentityID, f.agents[1].IdentityID}, "")
		before, err := conversationaction.NewListConversationMessagesQuery(db).Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID})
		if err != nil {
			t.Fatal(err)
		}
		silent := f.submitReply(t, agentruntime.RunResult{Outcome: agentruntime.RunOutcomeSilent}, nil)
		after, err := conversationaction.NewListConversationMessagesQuery(db).Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID})
		if err != nil {
			t.Fatal(err)
		}
		if len(after.Messages) != len(before.Messages) {
			t.Fatalf("静默结果不应写入消息：%d -> %d", len(before.Messages), len(after.Messages))
		}
		run := &servermodels.AgentRun{}
		if err := db.NewSelect().Model(run).
			Where("agr.conversation_id = ? AND agr.agent_identity_id = ?", f.groupID, silent).
			OrderExpr("agr.created_at DESC").Limit(1).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if run.Status != string(domain.AgentRunStatusSucceeded) || run.Outcome == nil ||
			*run.Outcome != string(domain.AgentRunOutcomeSilent) || run.ResponseMessageID != nil {
			t.Fatalf("静默运行 = %+v", run)
		}
		next := f.activeRun(t)
		if next == nil || next.AgentIdentityID == silent {
			t.Fatalf("静默后未轮转到另一位 AI 员工：%+v", next)
		}
	})
}
