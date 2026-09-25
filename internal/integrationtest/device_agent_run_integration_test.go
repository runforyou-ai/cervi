//go:build server

package integrationtest

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	deviceaction "github.com/runforyou-ai/cervi/internal/actions/device"
	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// deviceRunFixture 是设备执行集成测试共用的助理、设备与调用入口。
type deviceRunFixture struct {
	t         *testing.T
	ctx       context.Context
	db        *bun.DB
	identity  *servermodels.Identity
	assistant *agentaction.Assistant
	tasks     *servertask.Runtime
	executor  *agentrunaction.ExecuteAction
	sendFirst *conversationaction.SendFirstAgentTextMessageAction
	send      *conversationaction.SendAgentTextMessageAction
	device    agentrunaction.RunDevice
}

// testDeviceAgentRuns 验证助理的运行派发到其绑定电脑，以及领取、并行、停止、租约过期、换电脑、暂停与失去执行条件的收敛；AI 员工的运行始终在服务端执行。
func testDeviceAgentRuns(t *testing.T, db *bun.DB, identity *servermodels.Identity, employee *agentaction.Agent, tasks *servertask.Runtime) {
	ctx := context.Background()
	installID := uuid.NewV7().String()
	registered, err := deviceaction.NewRegisterDeviceAction(db).Execute(ctx, identity, deviceaction.RegisterInput{InstallID: installID, Name: "测试电脑", Platform: domain.DevicePlatformMacOS})
	if err != nil {
		t.Fatal(err)
	}
	assistant, err := agentaction.NewCreateAssistantAction(db).Execute(ctx, identity, registered.ID, agentaction.AssistantInput{
		DisplayName: "小码", Execution: agentaction.ManagedExecutionInput{
			ProviderID: employee.Execution.Managed.ProviderID, ModelIdentifier: employee.Execution.Managed.ModelIdentifier,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture := &deviceRunFixture{
		t: t, ctx: ctx, db: db, identity: identity, assistant: assistant, tasks: tasks,
		executor:  agentrunaction.NewExecuteAction(db, tasks, nil, testAttachmentReader(db), nil, nil),
		sendFirst: conversationaction.NewSendFirstAgentTextMessageAction(db, agentrunaction.NewScheduler(tasks)),
		send:      conversationaction.NewSendAgentTextMessageAction(db, agentrunaction.NewScheduler(tasks)),
		device:    agentrunaction.RunDevice{OrganizationID: identity.Organization.ID, UserID: identity.User.ID, DeviceID: registered.ID},
	}
	t.Run("助理归属主人", func(t *testing.T) {
		if assistant.OwnerUserID != identity.User.ID || assistant.DeviceID != registered.ID || assistant.Presence(time.Now()) != domain.AssistantPresenceOffline {
			t.Fatalf("assistant=%+v", assistant)
		}
		other := newChatLockUser(t, db, identity)
		// 其他成员不能在别人的电脑上创建助理，也不能与别人的助理单聊。
		if _, err := agentaction.NewCreateAssistantAction(db).Execute(ctx, other, registered.ID, agentaction.AssistantInput{
			DisplayName: "冒用", Execution: agentaction.ManagedExecutionInput{ProviderID: employee.Execution.Managed.ProviderID, ModelIdentifier: employee.Execution.Managed.ModelIdentifier},
		}); !errors.Is(err, agentaction.ErrAssistantDeviceNotFound) {
			t.Fatalf("create on foreign device=%v", err)
		}
		if _, err := fixture.sendFirst.Execute(ctx, other, conversationaction.FirstAgentTextMessageInput{
			ConversationID: uuid.NewV7().String(), AgentIdentityID: assistant.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "借用",
		}); !errors.Is(err, conversationaction.ErrAgentTargetNotFound) {
			t.Fatalf("foreign assistant chat=%v", err)
		}
		// AI 员工管理入口不能修改助理。
		if _, err := agentaction.NewUpdateStatusAction(db, testServiceSessionReturner(db)).Execute(ctx, identity, assistant.ID, domain.UserStatusInactive); err == nil {
			t.Fatal("agent status action changed assistant")
		}
		if _, err := agentaction.NewGetAgentQuery(db).Execute(ctx, identity, assistant.ID); !errors.Is(err, agentaction.ErrNotFound) {
			t.Fatalf("agent query read assistant=%v", err)
		}
	})

	t.Run("AI 员工在服务端执行", func(t *testing.T) {
		_, run := createAgentLockChat(t, ctx, db, identity, employee.IdentityID, tasks)
		if run.ExecutionDeviceID != nil {
			t.Fatalf("employee run dispatched to device=%+v", run)
		}
		if _, err := fixture.executor.StopAgentReply(ctx, identity, run.ConversationID, run.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("设备认证", func(t *testing.T) {
		other := newChatLockUser(t, db, identity)
		if _, err := deviceaction.NewAuthenticateDeviceAction(db).Execute(ctx, other, registered.ID); !errors.Is(err, deviceaction.ErrNotFound) {
			t.Fatalf("other member device auth=%v", err)
		}
	})

	t.Run("草稿首发即派发到绑定电脑", func(t *testing.T) {
		conversationID := uuid.NewV7().String()
		if _, err := fixture.sendFirst.Execute(ctx, identity, conversationaction.FirstAgentTextMessageInput{
			ConversationID: conversationID, AgentIdentityID: assistant.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "首条就在本机执行",
		}); err != nil {
			t.Fatal(err)
		}
		var run servermodels.AgentRun
		if err := db.NewSelect().Model(&run).Where("agr.conversation_id = ?", conversationID).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if run.ExecutionDeviceID == nil || *run.ExecutionDeviceID != registered.ID {
			t.Fatalf("first run=%+v", run)
		}
		if count, err := db.NewSelect().Model((*servermodels.TaskRun)(nil)).Where("tr.idempotency_key = ?", "agent:"+run.ID).Count(ctx); err != nil || count != 0 {
			t.Fatalf("first device run enqueued server task=%d %v", count, err)
		}
		work, err := fixture.executor.DeviceWork(ctx, fixture.device)
		if err != nil || !deviceWorkContains(work, run.ID) {
			t.Fatalf("device work=%+v %v", work, err)
		}
		fixture.claimAndComplete(run.ID, "首条的回复")
	})

	t.Run("运行期知识检索与附件读取", func(t *testing.T) {
		base, err := knowledgeaction.NewCreateKnowledgeBaseAction(db).Execute(ctx, identity, newKnowledgeBaseInput(t, db, identity, "设备资料", domain.KnowledgeBaseCategoryStandard))
		if err != nil {
			t.Fatal(err)
		}
		bindKnowledge := func(ids []string) {
			t.Helper()
			if _, err := agentaction.NewUpdateAssistantAction(db).Execute(ctx, identity, assistant.ID, agentaction.AssistantInput{
				DisplayName: assistant.DisplayName, Execution: agentaction.ManagedExecutionInput{
					ProviderID: employee.Execution.Managed.ProviderID, ModelIdentifier: employee.Execution.Managed.ModelIdentifier, KnowledgeBaseIDs: ids,
				},
			}); err != nil {
				t.Fatal(err)
			}
		}
		bindKnowledge([]string{base.ID})
		defer bindKnowledge(nil)
		executor := agentrunaction.NewExecuteAction(db, tasks, nil, testAttachmentReader(db), testDeviceKnowledge{}, nil)
		conversationID := fixture.assistantChat()
		sent, err := conversationaction.NewSendAttachmentMessageAction(db, agentrunaction.NewScheduler(tasks)).Execute(ctx, identity, conversationaction.AttachmentMessageInput{
			ConversationID: conversationID, ClientMessageID: uuid.NewV7().String(), Body: "看看截图",
			FileID: uploadedAttachment(t, db, identity, "screen.png", "image/png"),
		})
		if err != nil {
			t.Fatal(err)
		}
		var run servermodels.AgentRun
		if err := db.NewSelect().Model(&run).Where("agr.conversation_id = ? AND agr.status = ?", conversationID, domain.AgentRunStatusQueued).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		// 领取前不能读取运行期资料。
		if _, err := executor.SearchDeviceRunKnowledge(ctx, fixture.device, run.ID, knowledgeretrieval.Request{Queries: []string{"退款"}}); !errors.Is(err, agentrunaction.ErrDeviceRunLeaseLost) {
			t.Fatalf("search before claim=%v", err)
		}
		claim, err := executor.ClaimDeviceRun(ctx, fixture.device, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		var assignment agentruntime.Assignment
		if err := json.Unmarshal(claim.Assignment, &assignment); err != nil || !slices.Contains(assignment.Tools, agentruntime.KnowledgeToolName) {
			t.Fatalf("assignment=%+v %v", assignment, err)
		}
		result, err := executor.SearchDeviceRunKnowledge(ctx, fixture.device, run.ID, knowledgeretrieval.Request{Queries: []string{"退款"}})
		if err != nil || len(result.Records) != 1 || result.Records[0].KnowledgeBaseID != base.ID || result.Records[0].Content != "设备资料：退款" {
			t.Fatalf("search=%+v %v", result, err)
		}
		content, err := executor.ReadDeviceRunAttachment(ctx, fixture.device, run.ID, sent.Message.ID)
		if err != nil || string(content) != "content:screen.png" {
			t.Fatalf("attachment=%q %v", content, err)
		}
		// 其他会话的附件与其他设备都读不到。
		if _, err := executor.ReadDeviceRunAttachment(ctx, fixture.device, run.ID, uuid.NewV7().String()); !errors.Is(err, agentrunaction.ErrAttachmentUnavailable) {
			t.Fatalf("foreign attachment=%v", err)
		}
		otherDevice := fixture.device
		otherDevice.DeviceID = uuid.NewV7().String()
		if _, err := executor.ReadDeviceRunAttachment(ctx, otherDevice, run.ID, sent.Message.ID); !errors.Is(err, agentrunaction.ErrDeviceRunNotFound) {
			t.Fatalf("other device attachment=%v", err)
		}
		fixture.complete(run.ID, "已查阅资料")
	})

	t.Run("设备运行下发全部本机工具", func(t *testing.T) {
		run := fixture.sendAndLoadRun(fixture.assistantChat(), "看看文件")
		claim, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		var assignment agentruntime.Assignment
		if err := json.Unmarshal(claim.Assignment, &assignment); err != nil {
			t.Fatal(err)
		}
		for _, name := range agentruntime.LocalTools() {
			if !slices.Contains(assignment.Tools, name) {
				t.Fatalf("device run tools=%v", assignment.Tools)
			}
		}
		fixture.complete(run.ID, "已查看")
	})

	t.Run("派发领取与并行", func(t *testing.T) {
		before := fixture.workSeq()
		conversationID := fixture.assistantChat()
		run := fixture.sendAndLoadRun(conversationID, "在本机执行")
		if run.ExecutionDeviceID == nil || *run.ExecutionDeviceID != registered.ID {
			t.Fatalf("device run=%+v", run)
		}
		if count, err := db.NewSelect().Model((*servermodels.TaskRun)(nil)).Where("tr.idempotency_key = ?", "agent:"+run.ID).Count(ctx); err != nil || count != 0 {
			t.Fatalf("device run enqueued server task=%d %v", count, err)
		}
		if fixture.workSeq() <= before {
			t.Fatal("device work sequence did not advance")
		}
		work, err := fixture.executor.DeviceWork(ctx, fixture.device)
		if err != nil || !deviceWorkContains(work, run.ID) {
			t.Fatalf("device work=%+v %v", work, err)
		}
		if _, err := fixture.executor.ClaimDeviceRun(ctx, agentrunaction.RunDevice{OrganizationID: identity.Organization.ID, UserID: identity.User.ID, DeviceID: uuid.NewV7().String()}, run.ID); !errors.Is(err, agentrunaction.ErrDeviceRunNotFound) {
			t.Fatalf("foreign device claim=%v", err)
		}
		claim, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, run.ID)
		if err != nil || len(claim.Assignment) == 0 {
			t.Fatalf("claim=%+v %v", claim, err)
		}
		if _, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, run.ID); !errors.Is(err, agentrunaction.ErrDeviceRunUnavailable) {
			t.Fatalf("repeated claim=%v", err)
		}
		// 持有租约的设备取得配置版本锁定的模型服务，其他设备取不到。
		upstream, err := fixture.executor.ResolveDeviceModelUpstream(ctx, fixture.device, run.ID)
		if err != nil || upstream.Brand == "" || upstream.BaseURL == "" || upstream.Identifier == "" {
			t.Fatalf("model upstream=%+v %v", upstream, err)
		}
		if _, err := fixture.executor.ResolveDeviceModelUpstream(ctx, agentrunaction.RunDevice{OrganizationID: identity.Organization.ID, UserID: identity.User.ID, DeviceID: uuid.NewV7().String()}, run.ID); !errors.Is(err, agentrunaction.ErrDeviceRunNotFound) {
			t.Fatalf("foreign device model upstream=%v", err)
		}
		// 其他会话的运行不必等待，可以同时领取。
		waiting := fixture.sendAndLoadRun(fixture.assistantChat(), "并行")
		if _, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, waiting.ID); err != nil {
			t.Fatalf("parallel claim=%v", err)
		}
		fixture.complete(run.ID, "本机回复")
		if _, err := fixture.executor.ResolveDeviceModelUpstream(ctx, fixture.device, run.ID); !errors.Is(err, agentrunaction.ErrDeviceRunLeaseLost) {
			t.Fatalf("model upstream after completion=%v", err)
		}
		var message servermodels.Message
		if err := db.NewSelect().Model(&message).Where("msg.idempotency_key = ?", "agent:"+run.ID).Scan(ctx); err != nil || message.Body != "本机回复" {
			t.Fatalf("device reply=%+v %v", message, err)
		}
		// 已完成的运行再次上报保持幂等。
		if err := fixture.executor.CompleteDeviceRun(ctx, fixture.device, run.ID, agentruntime.RunResult{Content: "重复", EndSeq: 1}); err != nil {
			t.Fatal(err)
		}
		// 停止运行后续租得知结束，工作水位随之推进。
		stoppedSeq := fixture.workSeq()
		if status, err := fixture.executor.StopAgentReply(ctx, identity, waiting.ConversationID, waiting.ID); err != nil || status != domain.AgentRunStatusCancelled {
			t.Fatalf("stop=%s %v", status, err)
		}
		if fixture.workSeq() <= stoppedSeq {
			t.Fatal("stop did not advance device work sequence")
		}
		if lease, err := fixture.executor.RenewDeviceRunLease(ctx, fixture.device, waiting.ID); err != nil || !lease.Ended {
			t.Fatalf("lease after stop=%+v %v", lease, err)
		}
		// 停止后设备回传的过程内容仍被保留。
		if err := fixture.executor.FailDeviceRun(ctx, fixture.device, waiting.ID, domain.AgentRunErrorCodeDeviceRunFailed, "stopped", agentruntime.RunResult{Blocks: fixture.partialBlocks()}); err != nil {
			t.Fatal(err)
		}
		fixture.assertBlocks(waiting.ID, 1)
	})

	t.Run("设备运行输入状态", func(t *testing.T) {
		conversationID := fixture.assistantChat()
		run := fixture.sendAndLoadRun(conversationID, "输入状态")
		feed := startRealtimeFeed(t, identity.Organization.ID)
		if _, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, run.ID); err != nil {
			t.Fatal(err)
		}
		// 领取时建立助理聊天主体并开始发布正在输入。
		agentSubjectID := loadIdentitySubjectID(t, db, identity.Organization.ID, assistant.IdentityID)
		feed.expectTyping(t, feed.userTyping(identity.User.ID, conversationID, agentSubjectID, true))
		// 其他设备上报结果不影响本设备运行的输入状态。
		foreign := agentrunaction.RunDevice{OrganizationID: identity.Organization.ID, UserID: identity.User.ID, DeviceID: uuid.NewV7().String()}
		if err := fixture.executor.CompleteDeviceRun(ctx, foreign, run.ID, agentruntime.RunResult{Content: "他机", EndSeq: 1}); err == nil {
			t.Fatal("foreign device completed run")
		}
		if lease, err := fixture.executor.RenewDeviceRunLease(ctx, fixture.device, run.ID); err != nil || lease.Ended {
			t.Fatalf("renew=%+v %v", lease, err)
		}
		fixture.complete(run.ID, "本机回复")
		feed.expectTypingStopped(t, feed.userTyping(identity.User.ID, conversationID, agentSubjectID, false))
		// 收尾后迟到的续租不再开始发布。
		if lease, err := fixture.executor.RenewDeviceRunLease(ctx, fixture.device, run.ID); err != nil || !lease.Ended {
			t.Fatalf("renew after complete=%+v %v", lease, err)
		}
		feed.expectNoTyping(t)
	})
	t.Run("租约过期", func(t *testing.T) {
		run := fixture.sendAndLoadRun(fixture.assistantChat(), "租约")
		if _, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, run.ID); err != nil {
			t.Fatal(err)
		}
		if lease, err := fixture.executor.RenewDeviceRunLease(ctx, fixture.device, run.ID); err != nil || lease.Ended {
			t.Fatalf("renew=%+v %v", lease, err)
		}
		if _, err := db.NewUpdate().Model((*servermodels.AgentRun)(nil)).Set("lease_expires_at = now() - interval '1 second'").Where("id = ?", run.ID).Exec(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.executor.RenewDeviceRunLease(ctx, fixture.device, run.ID); !errors.Is(err, agentrunaction.ErrDeviceRunLeaseLost) {
			t.Fatalf("expired renew=%v", err)
		}
		// 扫描收敛前设备上报的迟到结果被拒绝，运行随即按租约过期收敛并保留过程内容。
		if err := fixture.executor.CompleteDeviceRun(ctx, fixture.device, run.ID, agentruntime.RunResult{Content: "迟到", EndSeq: 1, Blocks: fixture.partialBlocks()}); !errors.Is(err, agentrunaction.ErrDeviceRunLeaseLost) {
			t.Fatalf("expired complete=%v", err)
		}
		fixture.assertFailed(run.ID, domain.AgentRunErrorCodeDeviceLeaseExpired)
		fixture.assertBlocks(run.ID, 1)

		// 收尾请求等待会话锁期间租约过期，取得锁后按租约失效拒绝写入。
		waiting := fixture.sendAndLoadRun(fixture.assistantChat(), "等锁")
		if _, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, waiting.ID); err != nil {
			t.Fatal(err)
		}
		triggers, err := fixture.executor.PeekDeviceRunInputs(ctx, fixture.device, waiting.ID, 0)
		if err != nil || len(triggers) == 0 {
			t.Fatalf("peek=%+v %v", triggers, err)
		}
		claimed, err := fixture.executor.ClaimDeviceRunInputs(ctx, fixture.device, waiting.ID, triggers[len(triggers)-1].Seq)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.NewSelect().Model((*servermodels.Conversation)(nil)).Where("id = ?", waiting.ConversationID).For("UPDATE").Exists(ctx); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		completed := make(chan error, 1)
		go func() {
			completed <- fixture.executor.CompleteDeviceRun(ctx, fixture.device, waiting.ID, agentruntime.RunResult{Content: "等锁后的回复", EndSeq: claimed.Input.EndSeq})
		}()
		time.Sleep(200 * time.Millisecond)
		if _, err := db.NewUpdate().Model((*servermodels.AgentRun)(nil)).Set("lease_expires_at = now() - interval '1 second'").Where("id = ?", waiting.ID).Exec(ctx); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if err := <-completed; !errors.Is(err, agentrunaction.ErrDeviceRunLeaseLost) {
			t.Fatalf("complete after lease expired while waiting=%v", err)
		}
		fixture.sweep()
		fixture.assertFailed(waiting.ID, domain.AgentRunErrorCodeDeviceLeaseExpired)
		// 过期后同一执行范围的新输入另起运行，照常派发到设备。
		next := fixture.sendAndLoadRun(run.ConversationID, "继续")
		if next.ID == run.ID || next.Status != string(domain.AgentRunStatusQueued) {
			t.Fatalf("next run=%+v", next)
		}
		fixture.claimAndComplete(next.ID, "继续后的回复")
	})

	t.Run("超出总时限", func(t *testing.T) {
		expire := func(runID string) {
			if _, err := db.NewUpdate().Model((*servermodels.AgentRun)(nil)).
				Set("claimed_at = now() - make_interval(secs => ?)", agentrunaction.DeviceRunMaxDuration.Seconds()+1).
				Where("id = ?", runID).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		// 超出总时限后不再续租、不再代理模型请求，设备上报失败时按超时收敛并保留过程内容。
		run := fixture.sendAndLoadRun(fixture.assistantChat(), "超时")
		if claim, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, run.ID); err != nil || len(claim.Assignment) == 0 {
			t.Fatalf("claim=%+v %v", claim, err)
		}
		expire(run.ID)
		if _, err := fixture.executor.RenewDeviceRunLease(ctx, fixture.device, run.ID); !errors.Is(err, agentrunaction.ErrDeviceRunLeaseLost) {
			t.Fatalf("renew after deadline=%v", err)
		}
		if _, err := fixture.executor.ResolveDeviceModelUpstream(ctx, fixture.device, run.ID); !errors.Is(err, agentrunaction.ErrDeviceRunLeaseLost) {
			t.Fatalf("model upstream after deadline=%v", err)
		}
		if err := fixture.executor.FailDeviceRun(ctx, fixture.device, run.ID, domain.AgentRunErrorCodeDeviceRunFailed, "context canceled", agentruntime.RunResult{Blocks: fixture.partialBlocks()}); err != nil {
			t.Fatal(err)
		}
		fixture.assertFailed(run.ID, domain.AgentRunErrorCodeDeviceRunTimedOut)
		fixture.assertBlocks(run.ID, 1)

		// 租约仍在续期的运行超出总时限后由扫描收敛。
		swept := fixture.sendAndLoadRun(run.ConversationID, "扫描超时")
		if _, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, swept.ID); err != nil {
			t.Fatal(err)
		}
		expire(swept.ID)
		fixture.sweep()
		fixture.assertFailed(swept.ID, domain.AgentRunErrorCodeDeviceRunTimedOut)
	})

	t.Run("失败原因校验", func(t *testing.T) {
		run := fixture.sendAndLoadRun(fixture.assistantChat(), "失败")
		if err := fixture.executor.FailDeviceRun(ctx, fixture.device, run.ID, domain.AgentRunErrorCodeUserCancelled, "", agentruntime.RunResult{}); !errors.Is(err, agentrunaction.ErrDeviceRunFailureCodeInvalid) {
			t.Fatalf("invalid failure code=%v", err)
		}
		if err := fixture.executor.FailDeviceRun(ctx, fixture.device, run.ID, domain.AgentRunErrorCodeDeviceRunFailed, "", agentruntime.RunResult{}); err != nil {
			t.Fatal(err)
		}
		fixture.assertFailed(run.ID, domain.AgentRunErrorCodeDeviceRunFailed)
	})

	t.Run("暂停与恢复", func(t *testing.T) {
		conversationID := fixture.assistantChat()
		paused, err := agentaction.NewSetAssistantPausedAction(db).Execute(ctx, identity, assistant.ID, true)
		if err != nil || paused.Presence(time.Now()) != domain.AssistantPresencePaused {
			t.Fatalf("pause=%+v %v", paused, err)
		}
		_, err = fixture.send.Execute(ctx, identity, conversationaction.InternalTextMessageInput{ConversationID: conversationID, ClientMessageID: uuid.NewV7().String(), Body: "暂停中"})
		if conflict, ok := errors.AsType[*conversationaction.ConflictError](err); !ok || conflict.Reason != conversationaction.ConflictReasonAssistantPaused {
			t.Fatalf("send to paused assistant=%v", err)
		}
		if _, err := agentaction.NewSetAssistantPausedAction(db).Execute(ctx, identity, assistant.ID, false); err != nil {
			t.Fatal(err)
		}
		fixture.claimAndComplete(fixture.sendAndLoadRun(conversationID, "恢复后").ID, "恢复后的回复")
	})

	t.Run("换到另一台电脑", func(t *testing.T) {
		conversationID := fixture.assistantChat()
		queued := fixture.sendAndLoadRun(conversationID, "换电脑前")
		otherDevice, err := deviceaction.NewRegisterDeviceAction(db).Execute(ctx, identity, deviceaction.RegisterInput{InstallID: uuid.NewV7().String(), Name: "新电脑", Platform: domain.DevicePlatformLinux})
		if err != nil {
			t.Fatal(err)
		}
		moved, err := agentaction.NewMoveAssistantAction(db).Execute(ctx, identity, assistant.ID, otherDevice.ID)
		if err != nil || moved.DeviceID != otherDevice.ID {
			t.Fatalf("move=%+v %v", moved, err)
		}
		// 原电脑上未领取的运行被收敛。
		if _, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, queued.ID); !errors.Is(err, agentrunaction.ErrDeviceRunUnavailable) {
			t.Fatalf("claim on previous device=%v", err)
		}
		fixture.sweep()
		fixture.assertFailed(queued.ID, domain.AgentRunErrorCodeExecutionChanged)
		next := fixture.sendAndLoadRun(conversationID, "换电脑后")
		if next.ExecutionDeviceID == nil || *next.ExecutionDeviceID != otherDevice.ID {
			t.Fatalf("run after move=%+v", next)
		}
		if _, err := fixture.executor.StopAgentReply(ctx, identity, conversationID, next.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := agentaction.NewMoveAssistantAction(db).Execute(ctx, identity, assistant.ID, registered.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("撤销设备", func(t *testing.T) {
		run := fixture.sendAndLoadRun(fixture.assistantChat(), "撤销")
		if err := deviceaction.NewRevokeDeviceAction(db).Execute(ctx, identity, registered.ID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = deviceaction.NewRegisterDeviceAction(db).Execute(ctx, identity, deviceaction.RegisterInput{InstallID: installID, Name: "测试电脑", Platform: domain.DevicePlatformMacOS})
		})
		if _, err := deviceaction.NewAuthenticateDeviceAction(db).Execute(ctx, identity, registered.ID); !errors.Is(err, deviceaction.ErrNotFound) {
			t.Fatalf("revoked device auth=%v", err)
		}
		fixture.sweep()
		fixture.assertFailed(run.ID, domain.AgentRunErrorCodeDeviceUnavailable)
		// 绑定电脑撤销后，发送前即拒绝且未绑定优先于暂停，不再排队。
		if _, err := agentaction.NewSetAssistantPausedAction(db).Execute(ctx, identity, assistant.ID, true); err != nil {
			t.Fatal(err)
		}
		_, err := fixture.send.Execute(ctx, identity, conversationaction.InternalTextMessageInput{ConversationID: run.ConversationID, ClientMessageID: uuid.NewV7().String(), Body: "撤销后"})
		if conflict, ok := errors.AsType[*conversationaction.ConflictError](err); !ok || conflict.Reason != conversationaction.ConflictReasonAssistantUnbound {
			t.Fatalf("send to unbound assistant=%v", err)
		}
		reloaded, _, err := agentaction.NewGetAssistantQuery(db).Execute(ctx, identity, assistant.ID)
		if err != nil || reloaded.Presence(time.Now()) != domain.AssistantPresenceUnbound {
			t.Fatalf("reload assistant=%+v %v", reloaded, err)
		}
	})
}

// testDeviceKnowledge 为每个知识库提供一条以库名和查询拼成正文的检索结果。
type testDeviceKnowledge struct{}

// Sources 按知识库编号构造检索来源。
func (testDeviceKnowledge) Sources(_ context.Context, _ string, knowledgeBaseIDs []string) ([]knowledgeretrieval.Source, error) {
	sources := make([]knowledgeretrieval.Source, 0, len(knowledgeBaseIDs))
	for _, id := range knowledgeBaseIDs {
		sources = append(sources, knowledgeretrieval.Source{ID: id, Name: "设备资料", Retrieve: func(_ context.Context, query string) ([]knowledgeretrieval.Record, error) {
			return []knowledgeretrieval.Record{{KnowledgeBaseID: id, SegmentID: uuid.NewV7().String(), Content: "设备资料：" + query, Matched: true}}, nil
		}})
	}
	return sources, nil
}

// assistantChat 创建一条与测试助理的新单聊，停止首条消息的运行后返回会话编号。
func (f *deviceRunFixture) assistantChat() string {
	f.t.Helper()
	conversationID := uuid.NewV7().String()
	if _, err := f.sendFirst.Execute(f.ctx, f.identity, conversationaction.FirstAgentTextMessageInput{
		ConversationID: conversationID, AgentIdentityID: f.assistant.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "开始",
	}); err != nil {
		f.t.Fatalf("send first=%v", err)
	}
	var run servermodels.AgentRun
	if err := f.db.NewSelect().Model(&run).Where("agr.conversation_id = ?", conversationID).Scan(f.ctx); err != nil {
		f.t.Fatalf("load first run=%v", err)
	}
	if _, err := f.executor.StopAgentReply(f.ctx, f.identity, conversationID, run.ID); err != nil {
		f.t.Fatalf("stop first run=%v", err)
	}
	return conversationID
}

// sendAndLoadRun 在会话中发送一条消息并返回因此建立的排队运行。
func (f *deviceRunFixture) sendAndLoadRun(conversationID, body string) servermodels.AgentRun {
	f.t.Helper()
	if _, err := f.send.Execute(f.ctx, f.identity, conversationaction.InternalTextMessageInput{ConversationID: conversationID, ClientMessageID: uuid.NewV7().String(), Body: body}); err != nil {
		f.t.Fatal(err)
	}
	var run servermodels.AgentRun
	if err := f.db.NewSelect().Model(&run).Where("agr.conversation_id = ? AND agr.status = ?", conversationID, domain.AgentRunStatusQueued).Scan(f.ctx); err != nil {
		f.t.Fatal(err)
	}
	return run
}

// claimAndComplete 由设备领取运行并以指定正文收尾。
func (f *deviceRunFixture) claimAndComplete(runID, content string) {
	f.t.Helper()
	if _, err := f.executor.ClaimDeviceRun(f.ctx, f.device, runID); err != nil {
		f.t.Fatal(err)
	}
	f.complete(runID, content)
}

// complete 按设备协议读取并认领全部输入后以指定正文收尾。
func (f *deviceRunFixture) complete(runID, content string) {
	f.t.Helper()
	triggers, err := f.executor.PeekDeviceRunInputs(f.ctx, f.device, runID, 0)
	if err != nil || len(triggers) == 0 {
		f.t.Fatalf("peek=%+v %v", triggers, err)
	}
	claimed, err := f.executor.ClaimDeviceRunInputs(f.ctx, f.device, runID, triggers[len(triggers)-1].Seq)
	if err != nil || claimed.Suppressed || len(claimed.Input.Messages) == 0 {
		f.t.Fatalf("claim inputs=%+v %v", claimed, err)
	}
	if err := f.executor.CompleteDeviceRun(f.ctx, f.device, runID, agentruntime.RunResult{Content: content, EndSeq: claimed.Input.EndSeq}); err != nil {
		f.t.Fatal(err)
	}
	var run servermodels.AgentRun
	if err := f.db.NewSelect().Model(&run).Where("agr.id = ?", runID).Scan(f.ctx); err != nil || run.Status != string(domain.AgentRunStatusSucceeded) {
		f.t.Fatalf("completed run=%+v %v", run, err)
	}
}

// sweep 执行一次设备运行收敛扫描。
func (f *deviceRunFixture) sweep() {
	f.t.Helper()
	if err := f.executor.SweepDeviceRuns(f.ctx, struct{}{}); err != nil {
		f.t.Fatal(err)
	}
}

// assertFailed 核对运行以指定错误码失败并写入了失败消息。
func (f *deviceRunFixture) assertFailed(runID string, code domain.AgentRunErrorCode) {
	f.t.Helper()
	var run servermodels.AgentRun
	if err := f.db.NewSelect().Model(&run).Where("agr.id = ?", runID).Scan(f.ctx); err != nil {
		f.t.Fatal(err)
	}
	if run.Status != string(domain.AgentRunStatusFailed) || run.ErrorCode == nil || *run.ErrorCode != string(code) || run.ResponseMessageID == nil {
		f.t.Fatalf("failed run=%+v", run)
	}
}

// partialBlocks 构造设备回传的一个正文过程内容块。
func (f *deviceRunFixture) partialBlocks() []agentruntime.Block {
	return []agentruntime.Block{{
		ID: uuid.NewV7().String(), Position: 1, ModelCallID: uuid.NewV7().String(),
		Kind: domain.AgentRunBlockContent, Payload: agentruntime.BlockPayload{Text: "中断前的内容"},
	}}
}

// assertBlocks 核对运行保存的过程内容块数量。
func (f *deviceRunFixture) assertBlocks(runID string, expected int) {
	f.t.Helper()
	count, err := f.db.NewSelect().Model((*servermodels.AgentRunBlock)(nil)).Where("arb.agent_run_id = ?", runID).Count(f.ctx)
	if err != nil || count != expected {
		f.t.Fatalf("run blocks=%d %v", count, err)
	}
}

// workSeq 读取测试设备的工作水位。
func (f *deviceRunFixture) workSeq() int64 {
	f.t.Helper()
	var seq int64
	if err := f.db.NewSelect().Model((*servermodels.Device)(nil)).Column("work_seq").Where("d.id = ?", f.device.DeviceID).Scan(f.ctx, &seq); err != nil {
		f.t.Fatal(err)
	}
	return seq
}

// deviceWorkContains 判断待领取运行中是否包含指定运行。
func deviceWorkContains(work agentrunaction.DeviceWork, runID string) bool {
	for _, run := range work.Runs {
		if run.RunID == runID {
			return true
		}
	}
	return false
}
