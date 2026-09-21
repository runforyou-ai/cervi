//go:build server

package appservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	deviceaction "github.com/runforyou-ai/cervi/internal/actions/device"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

var _ DeviceRunBackend = (*DirectBackend)(nil)

// deviceIdentity 是已认证的请求设备及其登录身份。
type deviceIdentity struct {
	identity *servermodels.Identity
	device   agentrunaction.RunDevice
}

// AuthenticateDevice 校验实时事件流请求携带的登录令牌与本人未撤销设备，并返回当前身份。
func (b *DirectBackend) AuthenticateDevice(ctx context.Context, meta RequestMeta) (*servermodels.Identity, error) {
	device, err := b.ops.authenticateDevice(ctx, meta)
	if err != nil {
		return nil, err
	}
	return device.identity, nil
}

// authenticateDevice 先解析登录身份，再校验 DeviceHeader 指向本人未撤销的设备。
func (o *directOperations) authenticateDevice(ctx context.Context, meta RequestMeta) (deviceIdentity, error) {
	identity, err := o.authenticate(ctx, meta)
	if err != nil {
		return deviceIdentity{}, err
	}
	record, err := o.deviceAuthenticator.Execute(ctx, identity, meta.DeviceID)
	if errors.Is(err, deviceaction.ErrNotFound) {
		return deviceIdentity{}, NotFoundError(meta, cervii18n.ErrorDeviceNotFound)
	}
	if err != nil {
		if ctx.Err() != nil {
			return deviceIdentity{}, ctx.Err()
		}
		slog.Warn("设备认证失败", "organization_id", identity.Organization.ID, "error", err)
		return deviceIdentity{}, FailedError(meta, cervii18n.ErrorDeviceRunRequestFailed)
	}
	return deviceIdentity{identity: identity, device: agentrunaction.RunDevice{
		OrganizationID: identity.Organization.ID, UserID: identity.User.ID, DeviceID: record.ID,
	}}, nil
}

// GetDeviceWork 返回本设备的工作水位与待领取运行。
func (o *directOperations) GetDeviceWork(ctx context.Context, meta RequestMeta, device deviceIdentity) (DeviceWork, error) {
	work, err := o.agentCoordinator.DeviceWork(ctx, device.device)
	if err != nil {
		return DeviceWork{}, o.deviceRunError(ctx, meta, err, device, "")
	}
	output := DeviceWork{WorkSeq: work.WorkSeq, Runs: make([]DeviceWorkRun, 0, len(work.Runs))}
	for _, run := range work.Runs {
		output.Runs = append(output.Runs, DeviceWorkRun{RunID: run.RunID, ConversationID: run.ConversationID, WorkspaceID: run.WorkspaceID})
	}
	return output, nil
}

// ClaimDeviceRun 领取派发给本设备的排队运行并取得租约。
func (o *directOperations) ClaimDeviceRun(ctx context.Context, meta RequestMeta, device deviceIdentity, runID string) (DeviceRunClaim, error) {
	if !common.ValidUUID(runID) {
		return DeviceRunClaim{}, NotFoundError(meta, cervii18n.ErrorDeviceRunNotFound)
	}
	claim, err := o.agentCoordinator.ClaimDeviceRun(ctx, device.device, runID)
	if err != nil {
		return DeviceRunClaim{}, o.deviceRunError(ctx, meta, err, device, runID)
	}
	return DeviceRunClaim{
		Assignment: claim.Assignment, LeaseExpiresAt: claim.LeaseExpiresAt,
		LeaseRenewIntervalSeconds: int(agentrunaction.DeviceRunLeaseRenewInterval.Seconds()),
	}, nil
}

// RenewDeviceRunLease 为本设备持有的运行续租，运行已结束时返回 ended。
func (o *directOperations) RenewDeviceRunLease(ctx context.Context, meta RequestMeta, device deviceIdentity, runID string) (DeviceRunLease, error) {
	if !common.ValidUUID(runID) {
		return DeviceRunLease{}, NotFoundError(meta, cervii18n.ErrorDeviceRunNotFound)
	}
	lease, err := o.agentCoordinator.RenewDeviceRunLease(ctx, device.device, runID)
	if err != nil {
		return DeviceRunLease{}, o.deviceRunError(ctx, meta, err, device, runID)
	}
	if lease.Ended {
		return DeviceRunLease{Ended: true}, nil
	}
	return DeviceRunLease{LeaseExpiresAt: &lease.LeaseExpiresAt}, nil
}

// PeekDeviceRunInputs 返回本设备持有运行尚未认领的输入信号。
func (o *directOperations) PeekDeviceRunInputs(ctx context.Context, meta RequestMeta, device deviceIdentity, runID string, input DeviceRunInputPeekInput) (DeviceRunInputSignals, error) {
	if !common.ValidUUID(runID) {
		return DeviceRunInputSignals{}, NotFoundError(meta, cervii18n.ErrorDeviceRunNotFound)
	}
	triggers, err := o.agentCoordinator.PeekDeviceRunInputs(ctx, device.device, runID, int64(input.AfterSeq))
	if err != nil {
		return DeviceRunInputSignals{}, o.deviceRunError(ctx, meta, err, device, runID)
	}
	output := DeviceRunInputSignals{Seqs: make([]int64, 0, len(triggers))}
	for _, trigger := range triggers {
		output.Seqs = append(output.Seqs, trigger.Seq)
	}
	return output, nil
}

// ClaimDeviceRunInputs 为本设备持有的运行认领输入并返回截至该边界的上下文消息。
func (o *directOperations) ClaimDeviceRunInputs(ctx context.Context, meta RequestMeta, device deviceIdentity, runID string, input DeviceRunInputClaimInput) (DeviceRunClaimedInput, error) {
	if !common.ValidUUID(runID) {
		return DeviceRunClaimedInput{}, NotFoundError(meta, cervii18n.ErrorDeviceRunNotFound)
	}
	claimed, err := o.agentCoordinator.ClaimDeviceRunInputs(ctx, device.device, runID, input.ThroughSeq)
	if err != nil {
		return DeviceRunClaimedInput{}, o.deviceRunError(ctx, meta, err, device, runID)
	}
	if claimed.Suppressed {
		return DeviceRunClaimedInput{Suppressed: true, Messages: json.RawMessage("[]")}, nil
	}
	messages, err := json.Marshal(claimed.Input.Messages)
	if err != nil {
		return DeviceRunClaimedInput{}, o.deviceRunError(ctx, meta, fmt.Errorf("encode claimed messages: %w", err), device, runID)
	}
	return DeviceRunClaimedInput{EndSeq: claimed.Input.EndSeq, Messages: messages}, nil
}

// CompleteDeviceRun 以成功结果收尾本设备持有的运行。
func (o *directOperations) CompleteDeviceRun(ctx context.Context, meta RequestMeta, device deviceIdentity, runID string, input DeviceRunResultInput) error {
	if !common.ValidUUID(runID) {
		return NotFoundError(meta, cervii18n.ErrorDeviceRunNotFound)
	}
	result := agentruntime.RunResult{Content: input.Content, EndSeq: input.EndSeq}
	// 解码运行时透传的结束方式、用量与过程内容块，缺省表示直接回答且没有过程内容。
	for _, part := range []struct {
		data   json.RawMessage
		target any
	}{{input.Decision, &result.Decision}, {input.Usage, &result.Usage}, {input.Blocks, &result.Blocks}} {
		if len(part.data) == 0 || string(part.data) == "null" {
			continue
		}
		if err := json.Unmarshal(part.data, part.target); err != nil {
			slog.Warn("设备运行结果格式无效", "organization_id", device.device.OrganizationID, "device_id", device.device.DeviceID, "agent_run_id", runID, "error", err)
			return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
		}
	}
	if err := o.agentCoordinator.CompleteDeviceRun(ctx, device.device, runID, result); err != nil {
		return o.deviceRunError(ctx, meta, err, device, runID)
	}
	return nil
}

// FailDeviceRun 以失败原因收尾派发给本设备的运行。
func (o *directOperations) FailDeviceRun(ctx context.Context, meta RequestMeta, device deviceIdentity, runID string, input DeviceRunFailureInput) error {
	if !common.ValidUUID(runID) {
		return NotFoundError(meta, cervii18n.ErrorDeviceRunNotFound)
	}
	if err := o.agentCoordinator.FailDeviceRun(ctx, device.device, runID, domain.AgentRunErrorCode(input.ErrorCode), input.Message); err != nil {
		return o.deviceRunError(ctx, meta, err, device, runID)
	}
	return nil
}

// deviceRunError 转换设备运行期错误：运行不存在、不可领取、工作区忙与租约失效给出稳定原因码，其余记录日志后按请求失败收敛。
func (o *directOperations) deviceRunError(ctx context.Context, meta RequestMeta, err error, device deviceIdentity, runID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch {
	case errors.Is(err, agentrunaction.ErrDeviceRunNotFound):
		return NotFoundError(meta, cervii18n.ErrorDeviceRunNotFound)
	case errors.Is(err, agentrunaction.ErrDeviceRunUnavailable):
		return ConflictError(meta, cervii18n.ErrorDeviceRunUnavailable, "run_unavailable")
	case errors.Is(err, agentrunaction.ErrDeviceWorkspaceBusy):
		return ConflictError(meta, cervii18n.ErrorDeviceWorkspaceBusy, "workspace_busy")
	case errors.Is(err, agentrunaction.ErrDeviceRunLeaseLost):
		return ConflictError(meta, cervii18n.ErrorDeviceRunLeaseLost, "lease_lost")
	case errors.Is(err, agentrunaction.ErrDeviceRunFailureCodeInvalid):
		return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	slog.Warn("设备运行请求失败", "organization_id", device.device.OrganizationID, "device_id", device.device.DeviceID,
		"agent_run_id", runID, "error", err)
	return FailedError(meta, cervii18n.ErrorDeviceRunRequestFailed)
}
