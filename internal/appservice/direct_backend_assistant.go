//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"time"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// assistantOps 持有助理的 Action 和 Query。
type assistantOps struct {
	listAssistants        *agentaction.ListAssistantsQuery
	getAssistant          *agentaction.GetAssistantQuery
	createAssistant       *agentaction.CreateAssistantAction
	updateAssistant       *agentaction.UpdateAssistantAction
	setAssistantPaused    *agentaction.SetAssistantPausedAction
	moveAssistant         *agentaction.MoveAssistantAction
	updateAssistantStatus *agentaction.UpdateAssistantStatusAction
}

// newAssistantOps 创建助理的业务实现依赖。
func newAssistantOps(db *bun.DB) assistantOps {
	return assistantOps{
		listAssistants:        agentaction.NewListAssistantsQuery(db),
		getAssistant:          agentaction.NewGetAssistantQuery(db),
		createAssistant:       agentaction.NewCreateAssistantAction(db),
		updateAssistant:       agentaction.NewUpdateAssistantAction(db),
		setAssistantPaused:    agentaction.NewSetAssistantPausedAction(db),
		moveAssistant:         agentaction.NewMoveAssistantAction(db),
		updateAssistantStatus: agentaction.NewUpdateAssistantStatusAction(db),
	}
}

// assistantFieldKeys 是助理资料与执行配置的字段校验文案。
var assistantFieldKeys = map[common.FieldCode]cervii18n.Key{
	agentaction.ValidationDisplayNameRequired:      cervii18n.FieldAssistantNameRequired,
	agentaction.ValidationDisplayNameInvalid:       cervii18n.FieldDisplayNameInvalid,
	agentaction.ValidationExecutionInvalid:         cervii18n.FieldAgentExecutionInvalid,
	agentaction.ValidationKnowledgeBaseInvalid:     cervii18n.FieldAgentKnowledgeBaseInvalid,
	agentaction.ValidationModelInvalid:             cervii18n.FieldAgentModelInvalid,
	agentaction.ValidationSystemInstructionTooLong: cervii18n.FieldAgentSystemInstructionTooLong,
	agentaction.ValidationStatusInvalid:            cervii18n.FieldUserStatusInvalid,
}

// ListAssistants 返回当前成员名下的助理。
func (o *directOperations) ListAssistants(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (AssistantList, error) {
	return o.listOwnedAssistants(ctx, meta, identity, identity.User.ID)
}

// ListMemberAssistants 返回指定成员名下的助理。
func (o *directOperations) ListMemberAssistants(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, userID string) (AssistantList, error) {
	return o.listOwnedAssistants(ctx, meta, identity, userID)
}

// listOwnedAssistants 读取指定成员名下的助理并解析头像地址。
func (o *directOperations) listOwnedAssistants(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, userID string) (AssistantList, error) {
	records, err := o.listAssistants.Execute(ctx, identity, userID)
	if err != nil {
		return AssistantList{}, o.assistantError(ctx, meta, err, cervii18n.ErrorAssistantListFailed, identity.Organization.ID, "")
	}
	avatarFileIDs := make([]*string, 0, len(records))
	for _, record := range records {
		avatarFileIDs = append(avatarFileIDs, record.AvatarFileID)
	}
	avatarURLs, err := o.optionalFileURLs(ctx, identity, avatarFileIDs...)
	if err != nil {
		return AssistantList{}, o.assistantError(ctx, meta, err, cervii18n.ErrorAssistantListFailed, identity.Organization.ID, "")
	}
	now := time.Now()
	assistants := make([]Assistant, 0, len(records))
	for _, record := range records {
		assistants = append(assistants, assistantFromAction(record, optionalFileURL(avatarURLs, record.AvatarFileID), now))
	}
	return AssistantList{Assistants: assistants}, nil
}

// GetAssistant 返回当前成员名下的助理详情。
func (o *directOperations) GetAssistant(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, assistantID string) (AssistantDetail, error) {
	record, execution, err := o.getAssistant.Execute(ctx, identity, assistantID)
	if err != nil {
		return AssistantDetail{}, o.assistantError(ctx, meta, err, cervii18n.ErrorAssistantReadFailed, identity.Organization.ID, assistantID)
	}
	assistant, err := o.assistantWithAvatar(ctx, meta, identity, record, cervii18n.ErrorAssistantReadFailed)
	if err != nil {
		return AssistantDetail{}, err
	}
	// 转换助理的完整执行配置契约。
	var managed *AgentManagedExecution
	if execution.Managed != nil {
		managed = &AgentManagedExecution{
			ProviderID: execution.Managed.ProviderID, ProviderName: execution.Managed.ProviderName,
			ModelIdentifier: execution.Managed.ModelIdentifier, ModelName: execution.Managed.ModelName,
			SystemInstruction: execution.Managed.SystemInstruction, KnowledgeBaseIDs: execution.Managed.KnowledgeBaseIDs,
		}
	}
	return AssistantDetail{Assistant: assistant, Execution: AgentExecution{
		MCPServerIDs: execution.MCPServerIDs, RevisionID: execution.RevisionID, Mode: AgentExecutionMode(execution.Mode), Managed: managed,
	}}, nil
}

// CreateAssistant 在当前成员的电脑上创建助理。
func (o *directOperations) CreateAssistant(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input CreateAssistantInput) (Assistant, error) {
	record, err := o.createAssistant.Execute(ctx, identity, input.DeviceID, agentaction.AssistantInput{
		DisplayName: input.DisplayName, AvatarFileID: input.AvatarFileID, Execution: assistantExecutionInput(input.Execution),
	})
	if err != nil {
		return Assistant{}, o.assistantError(ctx, meta, err, cervii18n.ErrorAssistantCreateFailed, identity.Organization.ID, "")
	}
	return o.assistantWithAvatar(ctx, meta, identity, record, cervii18n.ErrorAssistantCreateFailed)
}

// UpdateAssistant 修改当前成员名下的助理。
func (o *directOperations) UpdateAssistant(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, assistantID string, input AssistantInput) (Assistant, error) {
	record, err := o.updateAssistant.Execute(ctx, identity, assistantID, agentaction.AssistantInput{
		DisplayName: input.DisplayName, AvatarFileID: input.AvatarFileID, Execution: assistantExecutionInput(input.Execution),
	})
	if err != nil {
		return Assistant{}, o.assistantError(ctx, meta, err, cervii18n.ErrorAssistantUpdateFailed, identity.Organization.ID, assistantID)
	}
	slog.Info("助理已保存", "organization_id", identity.Organization.ID, "assistant_id", assistantID, "revision_id", record.Execution.RevisionID)
	return o.assistantWithAvatar(ctx, meta, identity, record, cervii18n.ErrorAssistantUpdateFailed)
}

// PauseAssistant 暂停当前成员名下的助理。
func (o *directOperations) PauseAssistant(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, assistantID string) (Assistant, error) {
	return o.changeAssistantPaused(ctx, meta, identity, assistantID, true)
}

// ResumeAssistant 恢复当前成员名下已暂停的助理。
func (o *directOperations) ResumeAssistant(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, assistantID string) (Assistant, error) {
	return o.changeAssistantPaused(ctx, meta, identity, assistantID, false)
}

// changeAssistantPaused 修改助理的暂停状态。
func (o *directOperations) changeAssistantPaused(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, assistantID string, paused bool) (Assistant, error) {
	record, err := o.setAssistantPaused.Execute(ctx, identity, assistantID, paused)
	if err != nil {
		return Assistant{}, o.assistantError(ctx, meta, err, cervii18n.ErrorAssistantPauseFailed, identity.Organization.ID, assistantID)
	}
	slog.Info("助理暂停状态已修改", "organization_id", identity.Organization.ID, "assistant_id", assistantID, "paused", paused)
	return o.assistantWithAvatar(ctx, meta, identity, record, cervii18n.ErrorAssistantPauseFailed)
}

// MoveAssistant 把当前成员名下的助理换到指定电脑。
func (o *directOperations) MoveAssistant(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, assistantID string, input AssistantDeviceInput) (Assistant, error) {
	record, err := o.moveAssistant.Execute(ctx, identity, assistantID, input.DeviceID)
	if err != nil {
		return Assistant{}, o.assistantError(ctx, meta, err, cervii18n.ErrorAssistantMoveFailed, identity.Organization.ID, assistantID)
	}
	return o.assistantWithAvatar(ctx, meta, identity, record, cervii18n.ErrorAssistantMoveFailed)
}

// DeactivateAssistant 停用助理。
func (o *directOperations) DeactivateAssistant(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, assistantID string) (Assistant, error) {
	return o.changeAssistantStatus(ctx, meta, identity, assistantID, domain.UserStatusInactive)
}

// ReactivateAssistant 启用已停用的助理。
func (o *directOperations) ReactivateAssistant(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, assistantID string) (Assistant, error) {
	return o.changeAssistantStatus(ctx, meta, identity, assistantID, domain.UserStatusActive)
}

// changeAssistantStatus 修改助理的账号状态。
func (o *directOperations) changeAssistantStatus(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, assistantID string, status domain.UserStatus) (Assistant, error) {
	record, err := o.updateAssistantStatus.Execute(ctx, identity, assistantID, status)
	if err != nil {
		return Assistant{}, o.assistantError(ctx, meta, err, cervii18n.ErrorAssistantStatusUpdateFailed, identity.Organization.ID, assistantID)
	}
	slog.Info("助理状态已修改", "organization_id", identity.Organization.ID, "assistant_id", assistantID, "status", status)
	return o.assistantWithAvatar(ctx, meta, identity, record, cervii18n.ErrorAssistantStatusUpdateFailed)
}

// assistantWithAvatar 解析助理头像地址并转换契约。
func (o *directOperations) assistantWithAvatar(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, record *agentaction.Assistant, failureKey cervii18n.Key) (Assistant, error) {
	avatarURLs, err := o.optionalFileURLs(ctx, identity, record.AvatarFileID)
	if err != nil {
		return Assistant{}, o.assistantError(ctx, meta, err, failureKey, identity.Organization.ID, record.ID)
	}
	return assistantFromAction(*record, optionalFileURL(avatarURLs, record.AvatarFileID), time.Now()), nil
}

// assistantError 转换助理操作错误。
func (o *directOperations) assistantError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, assistantID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, assistantFieldKeys))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, agentaction.ErrAssistantNotFound) {
		return NotFoundError(meta, cervii18n.ErrorAssistantNotFound)
	}
	if errors.Is(err, agentaction.ErrAssistantDeviceNotFound) {
		return NotFoundError(meta, cervii18n.ErrorDeviceNotFound)
	}
	if errors.Is(err, agentaction.ErrAssistantOwnerInactive) {
		return ConflictError(meta, cervii18n.ErrorAssistantOwnerInactive, "assistant_owner_inactive")
	}
	if errors.Is(err, fileaction.ErrLinkedImageNotFound) {
		return NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}
	slog.Warn("助理操作失败", "organization_id", organizationID, "assistant_id", assistantID, "failure", failureKey, "error", err)
	return FailedError(meta, failureKey)
}

// assistantExecutionInput 转换助理的托管执行配置输入。
func assistantExecutionInput(input AgentManagedExecutionInput) agentaction.ManagedExecutionInput {
	return agentaction.ManagedExecutionInput{
		ProviderID: input.ProviderID, ModelIdentifier: input.ModelIdentifier,
		SystemInstruction: input.SystemInstruction, KnowledgeBaseIDs: input.KnowledgeBaseIDs,
	}
}

// assistantFromAction 转换助理契约并按当前时间计算在线状态。
func assistantFromAction(record agentaction.Assistant, avatarURL string, now time.Time) Assistant {
	var managed *AgentManagedExecutionSummary
	if record.Execution.Managed != nil {
		managed = &AgentManagedExecutionSummary{
			ProviderID: record.Execution.Managed.ProviderID, ProviderName: record.Execution.Managed.ProviderName,
			ModelIdentifier: record.Execution.Managed.ModelIdentifier, ModelName: record.Execution.Managed.ModelName,
		}
	}
	return Assistant{
		ID: record.ID, IdentityID: record.IdentityID, DisplayName: record.DisplayName, AvatarURL: avatarURL,
		Owner:    AssistantOwner{UserID: record.OwnerUserID, IdentityID: record.OwnerIdentityID, DisplayName: record.OwnerDisplayName},
		Device:   AssistantDevice{ID: record.DeviceID, Name: record.DeviceName},
		Status:   UserStatus(record.Status),
		Presence: AssistantPresence(record.Presence(now)),
		Execution: AgentExecutionSummary{
			RevisionID: record.Execution.RevisionID, Mode: AgentExecutionMode(record.Execution.Mode), Managed: managed,
		},
		CreatedAt: record.CreatedAt,
	}
}
