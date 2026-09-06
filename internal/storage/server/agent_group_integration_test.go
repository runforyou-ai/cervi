//go:build server

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type groupAgentFixture struct {
	t        *testing.T
	db       *bun.DB
	identity *servermodels.Identity
	groupID  string
	agents   []*agentaction.Agent
	subjects []string
	tasks    *servertask.Runtime
	send     *conversationaction.SendGroupTextMessageAction
}

// newGroupAgentFixture 建立包含两位 AI 员工的群聊和真实事务调度器。
func newGroupAgentFixture(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string) *groupAgentFixture {
	t.Helper()
	ctx := context.Background()
	f := &groupAgentFixture{t: t, db: db, identity: identity, tasks: servertask.New(db, serverconfig.NATSConfig{})}
	if err := f.tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := range 2 {
		agent, err := agentaction.NewCreateAgentAction(db).Execute(ctx, identity, agentaction.CreateInput{
			DisplayName: fmt.Sprintf("群助手 %d", i), RoleID: roleID,
			Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{
				ProviderID: providerID, ModelIdentifier: modelID, SystemInstruction: "按提醒回答群聊问题",
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		f.agents = append(f.agents, agent)
		ids = append(ids, agent.IdentityID)
	}
	group, err := conversationaction.NewCreateGroupConversationAction(db).Execute(ctx, identity, conversationaction.GroupConversationInput{Title: "AI 协作群", MemberIdentityIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	f.groupID = group.ID
	detail, err := conversationaction.NewGetGroupConversationQuery(db).Execute(ctx, identity, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range f.agents {
		for _, member := range detail.Participants {
			if member.IdentityID == agent.IdentityID {
				if member.IdentityType != domain.OrganizationIdentityTypeAgent {
					t.Fatalf("member=%+v", member)
				}
				f.subjects = append(f.subjects, member.ChatSubjectID)
			}
		}
	}
	f.send = conversationaction.NewSendGroupTextMessageAction(db, agentrunaction.NewScheduler(f.tasks))
	return f
}

// message 通过真实发送入口保存提醒及其运行事实。
func (f *groupAgentFixture) message(body string, all bool, subjects ...string) conversationaction.ConversationMessage {
	f.t.Helper()
	message, err := f.send.Execute(context.Background(), f.identity, conversationaction.GroupTextMessageInput{
		ConversationID: f.groupID, ClientMessageID: uuid.NewV7().String(), Body: body, MentionAll: all, MentionSubjectIDs: subjects,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return message
}

// queued 读取一位 Agent 的唯一排队运行。
func (f *groupAgentFixture) queued(index int) *servermodels.AgentRun {
	f.t.Helper()
	var runs []servermodels.AgentRun
	if err := f.db.NewSelect().Model(&runs).Where("agr.conversation_id = ? AND agr.agent_identity_id = ? AND agr.status = ?", f.groupID, f.agents[index].IdentityID, domain.AgentRunStatusQueued).Scan(context.Background()); err != nil {
		f.t.Fatal(err)
	}
	if len(runs) != 1 {
		f.t.Fatalf("queued runs=%+v", runs)
	}
	return &runs[0]
}

// claim 通过持久输入流读取最新可消费的群聊上下文。
func claimGroupAgentInput(ctx context.Context, feed agentruntime.InputFeed) (agentruntime.ClaimedInput, error) {
	triggers, err := feed.Peek(ctx, 0)
	if err != nil {
		return agentruntime.ClaimedInput{}, err
	}
	if len(triggers) == 0 {
		return agentruntime.ClaimedInput{}, errors.New("missing group triggers")
	}
	return feed.Claim(ctx, triggers[len(triggers)-1].Seq)
}

// testAgentGroupMentions 覆盖群聊提醒调度、上下文和成员生命周期。
func testAgentGroupMentions(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string) {
	t.Run("群主与企业边界", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, roleID, providerID, modelID)
		testGroupAgentBoundaries(t, f)
	})
	t.Run("显式提醒与所有人", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, roleID, providerID, modelID)
		testGroupAgentTriggers(t, f)
	})
	t.Run("多轮回复保持标准消息", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, roleID, providerID, modelID)
		testGroupAgentMessageHistory(t, f)
	})
	t.Run("移除后重新加入", func(t *testing.T) {
		f := newGroupAgentFixture(t, db, identity, roleID, providerID, modelID)
		testGroupAgentRemoval(t, f)
	})
	for _, scenario := range []string{"停用", "停用后恢复", "解散", "失败"} {
		t.Run(scenario, func(t *testing.T) {
			f := newGroupAgentFixture(t, db, identity, roleID, providerID, modelID)
			testGroupAgentTermination(t, f, scenario)
		})
	}
}

// testGroupAgentTriggers 验证去重、安全点补入、群序号、引用和最终回复幂等。
func testGroupAgentTriggers(t *testing.T, f *groupAgentFixture) {
	ctx := context.Background()
	ordinary := f.message("@群助手 0 手写提醒", false)
	count, err := f.db.NewSelect().Model((*servermodels.AgentRun)(nil)).Where("agr.conversation_id = ?", f.groupID).Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("ordinary runs=%d err=%v", count, err)
	}
	f.message("显式问题", false, f.subjects[0])
	input := conversationaction.GroupTextMessageInput{ConversationID: f.groupID, ClientMessageID: uuid.NewV7().String(), Body: "所有人问题", MentionAll: true, MentionSubjectIDs: []string{f.subjects[0]}, ReplyToMessageID: ordinary.ID}
	message, err := f.send.Execute(ctx, f.identity, input)
	if err != nil {
		t.Fatal(err)
	}
	if replay, err := f.send.Execute(ctx, f.identity, input); err != nil || replay.ID != message.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	count, err = f.db.NewSelect().Model((*servermodels.ConversationAgentTrigger)(nil)).Where("cat.trigger_message_id = ?", message.ID).Count(ctx)
	if err != nil || count != 2 {
		t.Fatalf("all mention triggers=%d err=%v", count, err)
	}
	// 即使触发消息的时间被调整到历史位置，群上下文仍以序号为界。
	if _, err := f.db.NewUpdate().Model((*servermodels.Message)(nil)).Set("originated_at = originated_at - interval '1 day'").Where("id = ?", message.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	calls := 0
	runtime := testAgentRuntime{run: func(ctx context.Context, request agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		calls++
		claimed, err := claimGroupAgentInput(ctx, feed)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		message := claimed.Messages[len(claimed.Messages)-1]
		if message.Role != agentruntime.MessageRoleUser || message.Name != f.identity.OrganizationIdentity.ID || !strings.HasSuffix(message.Content, "\n\n"+input.Body) {
			t.Fatalf("context=%+v", message)
		}
		for _, expected := range []string{"发言者：", "提醒：所有人", "提醒身份：" + f.agents[0].IdentityID, "引用消息 " + ordinary.ID, ordinary.Body} {
			if !strings.Contains(message.Content, expected) {
				t.Fatalf("context missing %q: %+v", expected, message)
			}
		}
		if calls == 1 {
			if !strings.Contains(request.Instruction, f.agents[0].IdentityID) {
				t.Fatalf("instruction=%s", request.Instruction)
			}
			f.message("补充问题", false, f.subjects[0])
			f.message("本轮边界之后的普通消息", false)
			claimed, err = claimGroupAgentInput(ctx, feed)
			if err != nil {
				return agentruntime.RunResult{}, err
			}
			if claimed.EndSeq != 3 || strings.Contains(claimed.Messages[len(claimed.Messages)-1].Content, "边界之后") {
				t.Fatalf("claimed=%+v", claimed)
			}
		}
		return agentruntime.RunResult{Content: "@所有人 回答完成", EndSeq: claimed.EndSeq, Usage: agentruntime.Usage{PromptTokens: 10, CompletionTokens: 5}}, nil
	}}
	execute := agentrunaction.NewExecuteAction(f.db, f.tasks, runtime, nil)
	for i := range 2 {
		run := f.queued(i)
		for range 2 {
			if err := execute.Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
				t.Fatal(err)
			}
		}
		if i == 0 {
			window, err := conversationaction.NewListConversationMessagesQuery(f.db).Execute(ctx, f.identity, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID})
			if err != nil || len(window.LatestAgentRuns) != 2 {
				t.Fatalf("group run states=%+v err=%v", window.LatestAgentRuns, err)
			}
			queued := 0
			for _, current := range window.LatestAgentRuns {
				if current.Status == domain.AgentRunStatusQueued {
					queued++
				}
			}
			if queued != 1 {
				t.Fatalf("remaining queued states=%+v", window.LatestAgentRuns)
			}
		}
	}
	if calls != 2 {
		t.Fatalf("runtime calls=%d", calls)
	}
	count, err = f.db.NewSelect().Model((*servermodels.ConversationAgentTrigger)(nil)).Where("cat.conversation_id = ?", f.groupID).Count(ctx)
	if err != nil || count != 4 {
		t.Fatalf("reply generated triggers=%d err=%v", count, err)
	}
	history, err := conversationaction.NewListConversationMessagesQuery(f.db).Execute(ctx, f.identity, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID})
	if err != nil {
		t.Fatal(err)
	}
	for _, reply := range history.Messages[len(history.Messages)-2:] {
		if reply.GroupMessageSequence == nil || reply.AgentProcess == nil || reply.AgentProcess.Usage.PromptTokens != 10 {
			t.Fatalf("reply=%+v", reply)
		}
	}
	// 引用 Agent 回复本身不形成新的触发。
	input.ClientMessageID, input.MentionAll, input.MentionSubjectIDs, input.ReplyToMessageID = uuid.NewV7().String(), false, nil, history.Messages[len(history.Messages)-1].ID
	if _, err := f.send.Execute(ctx, f.identity, input); err != nil {
		t.Fatal(err)
	}
	count, err = f.db.NewSelect().Model((*servermodels.ConversationAgentTrigger)(nil)).Where("cat.conversation_id = ?", f.groupID).Count(ctx)
	if err != nil || count != 4 {
		t.Fatalf("reply-only triggers=%d err=%v", count, err)
	}
}

// testGroupAgentMessageHistory 验证当前 Agent 的历史回答原样输入，其他 Agent 按群成员区分。
func testGroupAgentMessageHistory(t *testing.T, f *groupAgentFixture) {
	ctx := context.Background()
	replies := []string{"晚上好呀！🌙", `{"body":"用户要求的 JSON","senderName":"示例"}`}
	for round, reply := range replies {
		f.message("请继续", true)
		for agentIndex := range 2 {
			runtime := testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
				claimed, err := claimGroupAgentInput(ctx, feed)
				if err != nil {
					return agentruntime.RunResult{}, err
				}
				own, other := 0, 0
				for _, message := range claimed.Messages {
					switch message.Name {
					case f.agents[agentIndex].IdentityID:
						if message.Role != agentruntime.MessageRoleAssistant || message.Content != replies[own] {
							t.Fatalf("own history=%+v", message)
						}
						own++
					case f.agents[1-agentIndex].IdentityID:
						if message.Role != agentruntime.MessageRoleUser || !strings.HasSuffix(message.Content, "\n\n"+replies[other]) {
							t.Fatalf("other history=%+v", message)
						}
						other++
					}
				}
				if own != round || other != round {
					t.Fatalf("round=%d own=%d other=%d", round, own, other)
				}
				return agentruntime.RunResult{Content: reply, EndSeq: claimed.EndSeq}, nil
			}}
			run := f.queued(agentIndex)
			if err := agentrunaction.NewExecuteAction(f.db, f.tasks, runtime, nil).Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// 再次提醒时，JSON 也是正文，不作为业务消息外壳解析。
	f.message("继续检查历史", false, f.subjects[0])
	execute := agentrunaction.NewExecuteAction(f.db, f.tasks, testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		claimed, err := claimGroupAgentInput(ctx, feed)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		message := claimed.Messages[4]
		if message.Role != agentruntime.MessageRoleAssistant || message.Name != f.agents[0].IdentityID || message.Content != replies[1] {
			t.Fatalf("JSON history=%+v", message)
		}
		return agentruntime.RunResult{Content: "完成", EndSeq: claimed.EndSeq}, nil
	}}, nil)
	if err := execute.Execute(ctx, agentrunaction.RunInput{RunID: f.queued(0).ID}); err != nil {
		t.Fatal(err)
	}
}

// testGroupAgentRemoval 验证移除和重新加入后旧运行不能回复或消费新提醒。
func testGroupAgentRemoval(t *testing.T, f *groupAgentFixture) {
	ctx := context.Background()
	f.message("开始处理", false, f.subjects[0])
	old := f.queued(0)
	runtime := testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		claimed, err := claimGroupAgentInput(ctx, feed)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		if _, err := conversationaction.NewRemoveGroupConversationMemberAction(f.db).Execute(ctx, f.identity, conversationaction.GroupConversationMemberInput{ConversationID: f.groupID, MemberIdentityID: f.agents[0].IdentityID}); err != nil {
			t.Fatal(err)
		}
		if _, err := conversationaction.NewAddGroupConversationMembersAction(f.db).Execute(ctx, f.identity, conversationaction.GroupConversationMembersInput{ConversationID: f.groupID, MemberIdentityIDs: []string{f.agents[0].IdentityID}}); err != nil {
			t.Fatal(err)
		}
		f.message("重新加入后的问题", false, f.subjects[0])
		return agentruntime.RunResult{Content: "旧运行迟到回复", EndSeq: claimed.EndSeq}, nil
	}}
	execute := agentrunaction.NewExecuteAction(f.db, f.tasks, runtime, nil)
	if err := execute.Execute(ctx, agentrunaction.RunInput{RunID: old.ID}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.NewSelect().Model(old).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if old.Status != string(domain.AgentRunStatusCancelled) || old.ResponseMessageID != nil {
		t.Fatalf("old run=%+v", old)
	}
	next := f.queued(0)
	if next.ID == old.ID || next.TriggerStartSeq != 2 {
		t.Fatalf("next run=%+v", next)
	}
	execute = agentrunaction.NewExecuteAction(f.db, f.tasks, testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		claimed, err := claimGroupAgentInput(ctx, feed)
		return agentruntime.RunResult{Content: "新运行回复", EndSeq: claimed.EndSeq}, err
	}}, nil)
	if err := execute.Execute(ctx, agentrunaction.RunInput{RunID: next.ID}); err != nil {
		t.Fatal(err)
	}
}

// testGroupAgentTermination 验证模型调用期间停用、解散或失败时的确定终态。
func testGroupAgentTermination(t *testing.T, f *groupAgentFixture, scenario string) {
	ctx := context.Background()
	f.message("处理中的问题", false, f.subjects[0])
	run := f.queued(0)
	execute := agentrunaction.NewExecuteAction(f.db, f.tasks, testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		claimed, err := claimGroupAgentInput(ctx, feed)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		switch scenario {
		case "停用", "停用后恢复":
			_, err = agentaction.NewUpdateStatusAction(f.db).Execute(ctx, f.identity, f.agents[0].ID, domain.UserStatusInactive)
			if err == nil && scenario == "停用后恢复" {
				_, err = agentaction.NewUpdateStatusAction(f.db).Execute(ctx, f.identity, f.agents[0].ID, domain.UserStatusActive)
			}
		case "解散":
			err = conversationaction.NewLeaveGroupConversationAction(f.db).Execute(ctx, f.identity, conversationaction.GroupConversationLeaveInput{ConversationID: f.groupID})
		case "失败":
			return agentruntime.RunResult{}, errors.New("model unavailable")
		}
		if err != nil {
			t.Fatal(err)
		}
		return agentruntime.RunResult{Content: "不应写入的回答", EndSeq: claimed.EndSeq}, nil
	}}, nil)
	err := execute.Execute(ctx, agentrunaction.RunInput{RunID: run.ID})
	if scenario == "失败" {
		if err == nil {
			t.Fatal("missing runtime failure")
		}
	} else if err != nil {
		t.Fatal(err)
	}
	if err := f.db.NewSelect().Model(run).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	want := domain.AgentRunStatusCancelled
	if scenario == "失败" {
		want = domain.AgentRunStatusFailed
	}
	if run.Status != string(want) || run.ResponseMessageID != nil {
		t.Fatalf("terminal run=%+v", run)
	}
	if scenario != "解散" {
		f.message("后续所有人问题", true)
		if scenario == "失败" || scenario == "停用后恢复" {
			f.queued(0)
		}
		f.queued(1)
	}
}

// testGroupAgentBoundaries 验证 Agent 入群权限、群主限制和跨企业提醒隔离。
func testGroupAgentBoundaries(t *testing.T, f *groupAgentFixture) {
	ctx := context.Background()
	_, err := conversationaction.NewTransferGroupConversationOwnerAction(f.db).Execute(ctx, f.identity, conversationaction.GroupConversationOwnerInput{ConversationID: f.groupID, OwnerIdentityID: f.agents[0].IdentityID})
	var conflict *conversationaction.ConflictError
	if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonGroupMemberNotActive {
		t.Fatalf("agent owner error=%v", err)
	}
	foreign := newNavigationFixture(t)
	_, err = conversationaction.NewCreateGroupConversationAction(f.db).Execute(ctx, foreign.owner, conversationaction.GroupConversationInput{Title: "跨企业 Agent", MemberIdentityIDs: []string{f.agents[0].IdentityID}})
	if !errors.Is(err, conversationaction.ErrGroupMemberNotFound) {
		t.Fatalf("foreign create error=%v", err)
	}
	_, err = conversationaction.NewAddGroupConversationMembersAction(f.db).Execute(ctx, foreign.owner, conversationaction.GroupConversationMembersInput{ConversationID: foreign.groupID, MemberIdentityIDs: []string{f.agents[0].IdentityID}})
	if !errors.Is(err, conversationaction.ErrGroupMemberNotFound) {
		t.Fatalf("foreign add error=%v", err)
	}
	_, err = f.send.Execute(ctx, foreign.owner, conversationaction.GroupTextMessageInput{ConversationID: foreign.groupID, ClientMessageID: uuid.NewV7().String(), Body: "越界提及", MentionSubjectIDs: f.subjects})
	if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonGroupMentionTargetInvalid {
		t.Fatalf("foreign mention error=%v", err)
	}
	_, err = conversationaction.NewListConversationMessagesQuery(f.db).Execute(ctx, foreign.owner, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID})
	if !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("foreign history error=%v", err)
	}
	if _, err := agentaction.NewUpdateStatusAction(f.db).Execute(ctx, f.identity, f.agents[0].ID, domain.UserStatusInactive); err != nil {
		t.Fatal(err)
	}
	_, err = conversationaction.NewCreateGroupConversationAction(f.db).Execute(ctx, f.identity, conversationaction.GroupConversationInput{Title: "停用 Agent", MemberIdentityIDs: []string{f.agents[0].IdentityID}})
	if !errors.Is(err, conversationaction.ErrGroupMemberNotFound) {
		t.Fatalf("inactive create error=%v", err)
	}
}
