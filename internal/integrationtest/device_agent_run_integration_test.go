//go:build server

package integrationtest

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	deviceaction "github.com/runforyou-ai/cervi/internal/actions/device"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// deviceRunFixture 是设备执行集成测试共用的设备、工作区与调用入口。
type deviceRunFixture struct {
	t        *testing.T
	ctx      context.Context
	db       *bun.DB
	identity *servermodels.Identity
	agentID  string
	tasks    *servertask.Runtime
	executor *agentrunaction.ExecuteAction
	binding  *deviceaction.ConversationBindingAction
	send     *conversationaction.SendAgentTextMessageAction
	device   agentrunaction.RunDevice
}

// testDeviceAgentRuns 验证绑定设备后的派发、领取、工作区串行、停止、租约过期与失去执行条件的收敛。
func testDeviceAgentRuns(t *testing.T, db *bun.DB, identity *servermodels.Identity, agentIdentityID string, tasks *servertask.Runtime) {
	ctx := context.Background()
	installID := uuid.NewV7().String()
	registered, err := deviceaction.NewRegisterDeviceAction(db).Execute(ctx, identity, deviceaction.RegisterInput{InstallID: installID, Name: "测试电脑", Platform: domain.DevicePlatformMacOS})
	if err != nil {
		t.Fatal(err)
	}
	fixture := &deviceRunFixture{
		t: t, ctx: ctx, db: db, identity: identity, agentID: agentIdentityID, tasks: tasks,
		executor: agentrunaction.NewExecuteAction(db, tasks, nil, testAttachmentReader(db), nil),
		binding:  deviceaction.NewConversationBindingAction(db),
		send:     conversationaction.NewSendAgentTextMessageAction(db, agentrunaction.NewScheduler(tasks)),
		device:   agentrunaction.RunDevice{OrganizationID: identity.Organization.ID, UserID: identity.User.ID, DeviceID: registered.ID},
	}
	register := deviceaction.NewRegisterWorkspaceAction(db)
	first, err := register.Execute(ctx, identity, registered.ID, "cervi")
	if err != nil {
		t.Fatal(err)
	}
	second, err := register.Execute(ctx, identity, registered.ID, "docs")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("设备认证与工作区归属", func(t *testing.T) {
		other := newChatLockUser(t, db, identity)
		if _, err := deviceaction.NewAuthenticateDeviceAction(db).Execute(ctx, other, registered.ID); !errors.Is(err, deviceaction.ErrNotFound) {
			t.Fatalf("other member device auth=%v", err)
		}
		if _, err := register.Execute(ctx, other, registered.ID, "other"); !errors.Is(err, deviceaction.ErrNotFound) {
			t.Fatalf("other member workspace=%v", err)
		}
		conversationID := fixture.boundChat(first.ID)
		if _, err := fixture.binding.Bind(ctx, other, conversationID, first.ID); err == nil {
			t.Fatal("non-member bound conversation")
		}
	})

	t.Run("草稿首发即绑定工作区", func(t *testing.T) {
		sendFirst := conversationaction.NewSendFirstAgentTextMessageAction(db, agentrunaction.NewScheduler(tasks))
		// 工作区不属于本人设备时整个首发回滚，不留下会话。
		rejectedID := uuid.NewV7().String()
		if _, err := sendFirst.Execute(ctx, identity, conversationaction.FirstAgentTextMessageInput{
			ConversationID: rejectedID, AgentIdentityID: agentIdentityID, ClientMessageID: uuid.NewV7().String(), Body: "未知工作区", WorkspaceID: uuid.NewV7().String(),
		}); !errors.Is(err, deviceaction.ErrWorkspaceNotFound) {
			t.Fatalf("unknown workspace first send=%v", err)
		}
		if exists, err := db.NewSelect().Model((*servermodels.Conversation)(nil)).Where("cv.id = ?", rejectedID).Exists(ctx); err != nil || exists {
			t.Fatalf("rejected draft conversation exists=%v %v", exists, err)
		}

		conversationID := uuid.NewV7().String()
		if _, err := sendFirst.Execute(ctx, identity, conversationaction.FirstAgentTextMessageInput{
			ConversationID: conversationID, AgentIdentityID: agentIdentityID, ClientMessageID: uuid.NewV7().String(), Body: "首条就在本机执行", WorkspaceID: first.ID,
		}); err != nil {
			t.Fatal(err)
		}
		binding, err := fixture.binding.Get(ctx, identity, conversationID)
		if err != nil || binding == nil || binding.WorkspaceID != first.ID || binding.DeviceID != registered.ID {
			t.Fatalf("draft binding=%+v %v", binding, err)
		}
		var run servermodels.AgentRun
		if err := db.NewSelect().Model(&run).Where("agr.conversation_id = ?", conversationID).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if run.ExecutionDeviceID == nil || *run.ExecutionDeviceID != registered.ID || run.ExecutionWorkspaceID == nil || *run.ExecutionWorkspaceID != first.ID {
			t.Fatalf("first run=%+v", run)
		}
		if count, err := db.NewSelect().Model((*servermodels.TaskRun)(nil)).Where("tr.idempotency_key = ?", "agent:"+run.ID).Count(ctx); err != nil || count != 0 {
			t.Fatalf("first device run enqueued server task=%d %v", count, err)
		}
		if _, err := fixture.executor.StopAgentReply(ctx, identity, conversationID, run.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("派发领取与工作区串行", func(t *testing.T) {
		before := fixture.workSeq()
		conversationID := fixture.boundChat(first.ID)
		run := fixture.sendAndLoadRun(conversationID, "在本机执行")
		if run.ExecutionDeviceID == nil || *run.ExecutionDeviceID != registered.ID || run.ExecutionWorkspaceID == nil || *run.ExecutionWorkspaceID != first.ID {
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
		// 第二个会话绑定同一工作区，第一个运行结束前不能领取。
		waiting := fixture.sendAndLoadRun(fixture.boundChat(first.ID), "排队")
		if _, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, waiting.ID); !errors.Is(err, agentrunaction.ErrDeviceWorkspaceBusy) {
			t.Fatalf("busy workspace claim=%v", err)
		}
		fixture.complete(run.ID, "本机回复")
		var message servermodels.Message
		if err := db.NewSelect().Model(&message).Where("msg.idempotency_key = ?", "agent:"+run.ID).Scan(ctx); err != nil || message.Body != "本机回复" {
			t.Fatalf("device reply=%+v %v", message, err)
		}
		// 已完成的运行再次上报保持幂等。
		if err := fixture.executor.CompleteDeviceRun(ctx, fixture.device, run.ID, agentruntime.RunResult{Content: "重复", EndSeq: 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, waiting.ID); err != nil {
			t.Fatalf("claim after release=%v", err)
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
	})

	t.Run("租约过期", func(t *testing.T) {
		run := fixture.sendAndLoadRun(fixture.boundChat(second.ID), "租约")
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
		if err := fixture.executor.CompleteDeviceRun(ctx, fixture.device, run.ID, agentruntime.RunResult{Content: "迟到", EndSeq: 1}); !errors.Is(err, agentrunaction.ErrDeviceRunLeaseLost) {
			t.Fatalf("expired complete=%v", err)
		}
		fixture.sweep()
		fixture.assertFailed(run.ID, domain.AgentRunErrorCodeDeviceLeaseExpired)

		// 收尾请求等待会话锁期间租约过期，取得锁后按租约失效拒绝写入。
		waiting := fixture.sendAndLoadRun(fixture.boundChat(second.ID), "等锁")
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

	t.Run("工作区缺失与解绑", func(t *testing.T) {
		missing := fixture.sendAndLoadRun(fixture.boundChat(second.ID), "目录缺失")
		if err := fixture.executor.FailDeviceRun(ctx, fixture.device, missing.ID, domain.AgentRunErrorCodeUserCancelled, ""); !errors.Is(err, agentrunaction.ErrDeviceRunFailureCodeInvalid) {
			t.Fatalf("invalid failure code=%v", err)
		}
		if err := fixture.executor.FailDeviceRun(ctx, fixture.device, missing.ID, domain.AgentRunErrorCodeWorkspaceMissing, ""); err != nil {
			t.Fatal(err)
		}
		fixture.assertFailed(missing.ID, domain.AgentRunErrorCodeWorkspaceMissing)

		unbound := fixture.sendAndLoadRun(fixture.boundChat(second.ID), "解绑")
		version := conversationVersion(t, ctx, db, unbound.ConversationID)
		if err := fixture.binding.Unbind(ctx, identity, unbound.ConversationID); err != nil {
			t.Fatal(err)
		}
		if conversationVersion(t, ctx, db, unbound.ConversationID) <= version {
			t.Fatal("unbind did not advance conversation version")
		}
		if work, err := fixture.executor.DeviceWork(ctx, fixture.device); err != nil || deviceWorkContains(work, unbound.ID) {
			t.Fatalf("unbound run still offered=%+v %v", work, err)
		}
		if _, err := fixture.executor.ClaimDeviceRun(ctx, fixture.device, unbound.ID); !errors.Is(err, agentrunaction.ErrDeviceRunUnavailable) {
			t.Fatalf("unbound claim=%v", err)
		}
		fixture.sweep()
		fixture.assertFailed(unbound.ID, domain.AgentRunErrorCodeDeviceUnbound)
		serverRun := fixture.sendAndLoadRun(unbound.ConversationID, "回到服务端")
		if serverRun.ExecutionDeviceID != nil {
			t.Fatalf("unbound conversation dispatched to device=%+v", serverRun)
		}
	})

	t.Run("撤销设备", func(t *testing.T) {
		run := fixture.sendAndLoadRun(fixture.boundChat(second.ID), "撤销")
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
	})
}

// boundChat 创建一条新的 AI 单聊，停止首个服务端运行后绑定到指定工作区，返回会话编号。
func (f *deviceRunFixture) boundChat(workspaceID string) string {
	f.t.Helper()
	_, run := createAgentLockChat(f.t, f.ctx, f.db, f.identity, f.agentID, f.tasks)
	if _, err := f.executor.StopAgentReply(f.ctx, f.identity, run.ConversationID, run.ID); err != nil {
		f.t.Fatal(err)
	}
	binding, err := f.binding.Bind(f.ctx, f.identity, run.ConversationID, workspaceID)
	if err != nil || binding == nil || binding.WorkspaceID != workspaceID || binding.DeviceID != f.device.DeviceID {
		f.t.Fatalf("bind=%+v %v", binding, err)
	}
	return run.ConversationID
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

// workSeq 读取测试设备的工作水位。
func (f *deviceRunFixture) workSeq() int64 {
	f.t.Helper()
	var seq int64
	if err := f.db.NewSelect().Model((*servermodels.Device)(nil)).Column("work_seq").Where("d.id = ?", f.device.DeviceID).Scan(f.ctx, &seq); err != nil {
		f.t.Fatal(err)
	}
	return seq
}

// conversationVersion 读取会话当前版本。
func conversationVersion(t *testing.T, ctx context.Context, db *bun.DB, conversationID string) int64 {
	t.Helper()
	var version int64
	if err := db.NewSelect().Model((*servermodels.Conversation)(nil)).Column("version").Where("id = ?", conversationID).Scan(ctx, &version); err != nil {
		t.Fatal(err)
	}
	return version
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
