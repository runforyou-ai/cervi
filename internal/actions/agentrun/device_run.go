//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

const (
	// DeviceRunSweepActionName 是设备运行收敛扫描任务名。
	DeviceRunSweepActionName = "agent.device_run_sweep"
	// DeviceRunLeaseTTL 是设备领取或续租后租约的有效时长。
	DeviceRunLeaseTTL = 45 * time.Second
	// DeviceRunLeaseRenewInterval 是设备续租的间隔。
	DeviceRunLeaseRenewInterval = 10 * time.Second
	// deviceRunSweepBatch 是单次扫描收敛的运行数量上限。
	deviceRunSweepBatch = 100
)

var (
	// ErrDeviceRunNotFound 表示运行不存在或不由当前设备执行。
	ErrDeviceRunNotFound = errors.New("device agent run not found")
	// ErrDeviceRunUnavailable 表示运行已不处于可领取状态。
	ErrDeviceRunUnavailable = errors.New("device agent run is not claimable")
	// ErrDeviceWorkspaceBusy 表示同一工作区已有运行中的运行。
	ErrDeviceWorkspaceBusy = errors.New("device workspace is busy")
	// ErrDeviceRunLeaseLost 表示设备已不持有运行的有效租约。
	ErrDeviceRunLeaseLost = errors.New("device agent run lease lost")
	// ErrDeviceRunFailureCodeInvalid 表示设备上报的失败原因不在允许范围内。
	ErrDeviceRunFailureCodeInvalid = errors.New("device agent run failure code is invalid")
)

// RunDevice 标识以本机设备身份调用运行期接口的已认证设备。
type RunDevice struct {
	OrganizationID string
	UserID         string
	DeviceID       string
}

// DeviceWorkRun 是设备待领取运行的摘要。
type DeviceWorkRun struct {
	RunID          string `bun:"id"`
	ConversationID string `bun:"conversation_id"`
	WorkspaceID    string `bun:"execution_workspace_id"`
}

// DeviceWork 是设备当前工作水位与待领取运行。
type DeviceWork struct {
	WorkSeq int64
	Runs    []DeviceWorkRun
}

// DeviceClaim 是设备领取运行后得到的有效配置与租约。
type DeviceClaim struct {
	Assignment     json.RawMessage
	LeaseExpiresAt time.Time
}

// DeviceLease 是设备续租结果，Ended 表示运行已结束，设备应停止执行。
type DeviceLease struct {
	Ended          bool
	LeaseExpiresAt time.Time
}

// DeviceClaimedInput 是设备认领输入的结果，Suppressed 表示运行已失效，设备应停止执行。
type DeviceClaimedInput struct {
	Suppressed bool
	Input      agentruntime.ClaimedInput
}

// DeviceWork 返回设备的工作水位与按创建顺序排列、会话绑定仍指向本设备的待领取运行。
func (a *ExecuteAction) DeviceWork(ctx context.Context, device RunDevice) (DeviceWork, error) {
	work := DeviceWork{Runs: make([]DeviceWorkRun, 0)}
	if err := a.db.NewSelect().Model((*servermodels.Device)(nil)).Column("work_seq").
		Where("d.organization_id = ? AND d.id = ?", device.OrganizationID, device.DeviceID).
		Scan(ctx, &work.WorkSeq); err != nil {
		return DeviceWork{}, fmt.Errorf("load device work sequence: %w", err)
	}
	if err := a.db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		Column("id", "conversation_id", "execution_workspace_id").
		// 会话绑定已解除或更换的运行不再交给设备，由收敛扫描标记失败。
		Join("JOIN conversation_device_bindings AS cdb ON cdb.organization_id = agr.organization_id AND cdb.conversation_id = agr.conversation_id").
		Where("cdb.device_id = agr.execution_device_id AND cdb.workspace_id = agr.execution_workspace_id").
		Where("agr.organization_id = ? AND agr.execution_device_id = ?", device.OrganizationID, device.DeviceID).
		Where("agr.status = ?", domain.AgentRunStatusQueued).
		OrderExpr("agr.created_at ASC, agr.id ASC").
		Scan(ctx, &work.Runs); err != nil {
		return DeviceWork{}, fmt.Errorf("list device queued agent runs: %w", err)
	}
	return work, nil
}

// ClaimDeviceRun 由执行设备领取排队中的运行：标记运行中并取得租约，再按领取设备的能力解析并固定有效配置。
func (a *ExecuteAction) ClaimDeviceRun(ctx context.Context, device RunDevice, runID string) (DeviceClaim, error) {
	initial, err := a.loadDeviceRun(ctx, device, runID)
	if err != nil {
		return DeviceClaim{}, err
	}
	if initial.Status != string(domain.AgentRunStatusQueued) {
		return DeviceClaim{}, ErrDeviceRunUnavailable
	}
	policy, err := a.policyForRun(ctx, initial)
	if err != nil {
		return DeviceClaim{}, err
	}
	claim := DeviceClaim{}
	err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		locked, err := lockAgentRun(ctx, tx, policy, initial)
		if err != nil {
			return err
		}
		run := locked.Run
		if run.Status != string(domain.AgentRunStatusQueued) {
			return ErrDeviceRunUnavailable
		}
		// 会话绑定已解除或更换时拒绝领取，由收敛扫描标记失败。
		bound, err := tx.NewSelect().Model((*servermodels.ConversationDeviceBinding)(nil)).
			Where("cdb.organization_id = ? AND cdb.conversation_id = ?", run.OrganizationID, run.ConversationID).
			Where("cdb.device_id = ? AND cdb.workspace_id = ?", run.ExecutionDeviceID, run.ExecutionWorkspaceID).
			Exists(ctx)
		if err != nil {
			return fmt.Errorf("check device run binding: %w", err)
		}
		if !bound {
			return ErrDeviceRunUnavailable
		}
		if err := tx.NewRaw(`
			UPDATE agent_runs
			SET status = ?, started_at = COALESCE(started_at, now()), claimed_at = now(),
				lease_expires_at = now() + make_interval(secs => ?), updated_at = now()
			WHERE id = ?
			RETURNING lease_expires_at
		`, domain.AgentRunStatusRunning, DeviceRunLeaseTTL.Seconds(), run.ID).Scan(ctx, &claim.LeaseExpiresAt); err != nil {
			if pgerr.UniqueViolationOn(err, "agent_runs_running_workspace_unique") {
				return ErrDeviceWorkspaceBusy
			}
			return fmt.Errorf("claim device agent run: %w", err)
		}
		// 运行期间以 AI 员工的聊天主体发布输入状态。
		if _, err := chatstate.EnsureOrganizationIdentityChatSubject(ctx, tx, run.OrganizationID, run.AgentIdentityID, uuid.NewV7().String()); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*servermodels.DeviceWorkspace)(nil)).
			Set("last_used_at = now()").Set("updated_at = now()").
			Where("organization_id = ? AND id = ?", run.OrganizationID, run.ExecutionWorkspaceID).
			Exec(ctx); err != nil {
			return fmt.Errorf("touch device workspace: %w", err)
		}
		return chatstate.TouchConversation(ctx, tx, locked.PolicyContext.Conversation)
	})
	if err != nil {
		return DeviceClaim{}, err
	}
	if claim.Assignment, err = a.deviceAssignment(ctx, runID, policy); err != nil {
		// 运行已转为运行中但设备拿不到有效配置，立即以失败结束，不留下无人执行的运行。
		if _, failErr := a.fail(context.WithoutCancel(ctx), runID, err, domain.AgentRunErrorCodeDeviceRunFailed); failErr != nil {
			return DeviceClaim{}, fmt.Errorf("resolve device agent run assignment: %v; fail run: %w", err, failErr)
		}
		return DeviceClaim{}, err
	}
	a.holdDeviceRunTyping(ctx, initial, claim.LeaseExpiresAt)
	slog.Info("设备已领取 Agent 运行", "organization_id", device.OrganizationID, "device_id", device.DeviceID,
		"agent_run_id", runID, "workspace_id", initial.ExecutionWorkspaceID)
	return claim, nil
}

// deviceAssignment 按领取设备的能力解析并固定已领取运行的有效配置，返回运行时的不透明 JSON。
func (a *ExecuteAction) deviceAssignment(ctx context.Context, runID string, policy agentRunPolicy) (json.RawMessage, error) {
	execution, terminal, err := a.loadExecution(ctx, runID)
	if err != nil {
		return nil, err
	}
	if terminal {
		return nil, ErrDeviceRunUnavailable
	}
	// 设备首版不加载企业远程 MCP 服务，知识检索经服务端执行。
	knowledge, err := loadRunKnowledgeSearch(ctx, a.db, a.knowledge, execution)
	if err != nil {
		return nil, fmt.Errorf("load device agent run knowledge bases: %w", err)
	}
	assignment, err := a.resolveAssignment(ctx, execution, policy, agentruntime.Capabilities{Knowledge: knowledge != nil})
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(assignment)
	if err != nil {
		return nil, fmt.Errorf("encode device agent run assignment: %w", err)
	}
	return encoded, nil
}

// RenewDeviceRunLease 为设备持有的运行续租；运行已结束时返回 Ended，租约已过期时返回 ErrDeviceRunLeaseLost。
func (a *ExecuteAction) RenewDeviceRunLease(ctx context.Context, device RunDevice, runID string) (DeviceLease, error) {
	run, err := a.loadDeviceRun(ctx, device, runID)
	if err != nil {
		return DeviceLease{}, err
	}
	if agentRunStatusTerminal(run.Status) {
		a.releaseDeviceRunTyping(runID)
		return DeviceLease{Ended: true}, nil
	}
	lease := DeviceLease{}
	err = a.db.NewRaw(`
		UPDATE agent_runs
		SET lease_expires_at = now() + make_interval(secs => ?), updated_at = now()
		WHERE id = ? AND status = ? AND lease_expires_at > clock_timestamp()
		RETURNING lease_expires_at
	`, DeviceRunLeaseTTL.Seconds(), run.ID, domain.AgentRunStatusRunning).Scan(ctx, &lease.LeaseExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		// 续租与停止或收敛并发时按最新状态判断。
		current, reloadErr := a.loadDeviceRun(ctx, device, runID)
		if reloadErr != nil {
			return DeviceLease{}, reloadErr
		}
		a.releaseDeviceRunTyping(runID)
		if agentRunStatusTerminal(current.Status) {
			return DeviceLease{Ended: true}, nil
		}
		return DeviceLease{}, ErrDeviceRunLeaseLost
	}
	if err != nil {
		return DeviceLease{}, fmt.Errorf("renew device agent run lease: %w", err)
	}
	a.holdDeviceRunTyping(ctx, run, lease.LeaseExpiresAt)
	return lease, nil
}

// PeekDeviceRunInputs 返回设备持有运行尚未进入 TurnLoop 缓冲区的连续输入信号。
func (a *ExecuteAction) PeekDeviceRunInputs(ctx context.Context, device RunDevice, runID string, afterSeq int64) ([]agentruntime.Trigger, error) {
	run, err := a.requireDeviceLease(ctx, device, runID)
	if err != nil {
		return nil, err
	}
	feed := &databaseInputFeed{db: a.db, execution: executionContext{Run: *run}}
	return feed.Peek(ctx, afterSeq)
}

// ClaimDeviceRunInputs 为设备持有的运行认领截至指定序号的输入，并按运行策略重建截至该边界的会话上下文。
func (a *ExecuteAction) ClaimDeviceRunInputs(ctx context.Context, device RunDevice, runID string, throughSeq int64) (DeviceClaimedInput, error) {
	run, err := a.requireDeviceLease(ctx, device, runID)
	if err != nil {
		return DeviceClaimedInput{}, err
	}
	policy, err := a.policyForRun(ctx, run)
	if err != nil {
		return DeviceClaimedInput{}, err
	}
	feed := &databaseInputFeed{db: a.db, enqueuer: a.enqueuer, execution: executionContext{Run: *run}, policy: policy, attachments: a.attachments}
	claimed, err := feed.Claim(withDeviceLease(ctx, device.DeviceID), throughSeq)
	if errors.Is(err, errAgentRunSuppressed) {
		return DeviceClaimedInput{Suppressed: true}, nil
	}
	if err != nil {
		return DeviceClaimedInput{}, err
	}
	return DeviceClaimedInput{Input: claimed}, nil
}

// CompleteDeviceRun 按设备上报的运行结果收尾；运行已进入终态时直接返回，重复上报保持幂等。
func (a *ExecuteAction) CompleteDeviceRun(ctx context.Context, device RunDevice, runID string, result agentruntime.RunResult) error {
	run, err := a.loadDeviceRun(ctx, device, runID)
	if err != nil {
		return err
	}
	if agentRunStatusTerminal(run.Status) {
		a.releaseDeviceRunTyping(runID)
		return nil
	}
	if !deviceLeaseValid(run) {
		return ErrDeviceRunLeaseLost
	}
	execution, terminal, err := a.loadExecution(ctx, runID)
	if err != nil {
		return err
	}
	if terminal {
		a.releaseDeviceRunTyping(runID)
		return nil
	}
	policy, err := a.policyForRun(ctx, run)
	if err != nil {
		return err
	}
	if err := a.complete(withDeviceLease(ctx, device.DeviceID), execution, policy, result); err != nil {
		return fmt.Errorf("persist completed device agent run: %w", err)
	}
	a.releaseDeviceRunTyping(runID)
	// 迟到结果被门禁抑制时运行已被取消，保留已产生的过程内容。
	return a.persistPartialProcess(ctx, &execution.Run, result)
}

// FailDeviceRun 按设备上报的失败原因收尾运行；排队中的运行可在领取前因工作区缺失失败，运行中的运行要求设备仍持有租约。
func (a *ExecuteAction) FailDeviceRun(ctx context.Context, device RunDevice, runID string, code domain.AgentRunErrorCode, message string) error {
	if code != domain.AgentRunErrorCodeWorkspaceMissing && code != domain.AgentRunErrorCodeDeviceRunFailed {
		return ErrDeviceRunFailureCodeInvalid
	}
	run, err := a.loadDeviceRun(ctx, device, runID)
	if err != nil {
		return err
	}
	if agentRunStatusTerminal(run.Status) {
		a.releaseDeviceRunTyping(runID)
		return nil
	}
	if run.Status == string(domain.AgentRunStatusRunning) && !deviceLeaseValid(run) {
		return ErrDeviceRunLeaseLost
	}
	if message == "" {
		message = string(code)
	}
	if _, err := a.fail(withDeviceLease(ctx, device.DeviceID), runID, errors.New(message), code); err != nil {
		return fmt.Errorf("fail device agent run: %w", err)
	}
	a.releaseDeviceRunTyping(runID)
	slog.Info("设备上报 Agent 运行失败", "organization_id", device.OrganizationID, "device_id", device.DeviceID,
		"agent_run_id", runID, "error_code", code)
	return nil
}

// SweepDeviceRuns 收敛无法继续的设备运行：运行中但租约已过期，或排队中但设备已撤销、设备主人已停用、会话绑定已变化。
func (a *ExecuteAction) SweepDeviceRuns(ctx context.Context, _ struct{}) error {
	var stale []struct {
		ID   string                   `bun:"id"`
		Code domain.AgentRunErrorCode `bun:"code"`
	}
	if err := a.db.NewRaw(`
		SELECT agr.id,
			CASE
				WHEN agr.status = ? THEN ?
				WHEN d.id IS NULL OR d.revoked_at IS NOT NULL OR u.id IS NULL THEN ?
				ELSE ?
			END AS code
		FROM agent_runs AS agr
		LEFT JOIN devices AS d ON d.id = agr.execution_device_id AND d.organization_id = agr.organization_id
		LEFT JOIN users AS u ON u.id = d.user_id AND u.organization_id = d.organization_id AND u.status = ?
		LEFT JOIN conversation_device_bindings AS cdb ON cdb.organization_id = agr.organization_id AND cdb.conversation_id = agr.conversation_id
		WHERE agr.execution_device_id IS NOT NULL
			AND (
				(agr.status = ? AND agr.lease_expires_at <= clock_timestamp())
				OR (agr.status = ? AND (
					d.id IS NULL OR d.revoked_at IS NOT NULL OR u.id IS NULL
					OR cdb.id IS NULL OR cdb.device_id <> agr.execution_device_id OR cdb.workspace_id <> agr.execution_workspace_id
				))
			)
		ORDER BY agr.created_at ASC
		LIMIT ?
	`, domain.AgentRunStatusRunning, domain.AgentRunErrorCodeDeviceLeaseExpired, domain.AgentRunErrorCodeDeviceUnavailable, domain.AgentRunErrorCodeDeviceUnbound,
		domain.UserStatusActive, domain.AgentRunStatusRunning, domain.AgentRunStatusQueued, deviceRunSweepBatch).Scan(ctx, &stale); err != nil {
		return fmt.Errorf("find stale device agent runs: %w", err)
	}
	for _, run := range stale {
		if _, err := a.fail(ctx, run.ID, errors.New(string(run.Code)), run.Code); err != nil {
			return fmt.Errorf("fail stale device agent run %s: %w", run.ID, err)
		}
		a.releaseDeviceRunTyping(run.ID)
		slog.Info("设备 Agent 运行已收敛为失败", "agent_run_id", run.ID, "error_code", run.Code)
	}
	return nil
}

// loadDeviceRun 读取由指定设备执行的运行。
func (a *ExecuteAction) loadDeviceRun(ctx context.Context, device RunDevice, runID string) (*servermodels.AgentRun, error) {
	run := &servermodels.AgentRun{}
	err := a.db.NewSelect().Model(run).
		Where("agr.organization_id = ? AND agr.id = ?", device.OrganizationID, runID).
		Where("agr.execution_device_id = ?", device.DeviceID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDeviceRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load device agent run: %w", err)
	}
	return run, nil
}

// requireDeviceLease 读取设备持有有效租约的运行中运行。
func (a *ExecuteAction) requireDeviceLease(ctx context.Context, device RunDevice, runID string) (*servermodels.AgentRun, error) {
	run, err := a.loadDeviceRun(ctx, device, runID)
	if err != nil {
		return nil, err
	}
	if !deviceLeaseValid(run) {
		return nil, ErrDeviceRunLeaseLost
	}
	return run, nil
}

type deviceLeaseKey struct{}

// withDeviceLease 标记本次调用来自执行设备，事务锁定运行后据此再次校验设备租约。
func withDeviceLease(ctx context.Context, deviceID string) context.Context {
	return context.WithValue(ctx, deviceLeaseKey{}, deviceID)
}

// checkDeviceLease 在事务锁定运行后校验设备调用：运行必须由该设备执行，运行中的运行要求租约尚未过期。
func checkDeviceLease(ctx context.Context, db bun.IDB, run *servermodels.AgentRun) error {
	deviceID, ok := ctx.Value(deviceLeaseKey{}).(string)
	if !ok {
		return nil
	}
	if run.ExecutionDeviceID == nil || *run.ExecutionDeviceID != deviceID {
		return ErrDeviceRunNotFound
	}
	if run.Status != string(domain.AgentRunStatusRunning) {
		return nil
	}
	valid, err := db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		Where("agr.id = ? AND agr.lease_expires_at > clock_timestamp()", run.ID).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("check device agent run lease: %w", err)
	}
	if !valid {
		return ErrDeviceRunLeaseLost
	}
	return nil
}

// deviceLeaseValid 判断运行处于运行中且设备租约尚未过期。
func deviceLeaseValid(run *servermodels.AgentRun) bool {
	return run.Status == string(domain.AgentRunStatusRunning) && run.LeaseExpiresAt != nil && run.LeaseExpiresAt.After(time.Now())
}
