//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// CreateAgent 创建企业 AI 员工。
func (b *DirectBackend) CreateAgent(ctx context.Context, meta RequestMeta, input CreateAgentInput) (Agent, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Agent{}, err
	}
	created, err := b.createAgent.Execute(ctx, identity, agentaction.CreateInput{
		DisplayName: input.DisplayName, TeamIDs: input.TeamIDs,
		Capability: agentCapabilityInput(input.Capability),
	})
	if err != nil {
		return Agent{}, b.agentError(ctx, meta, err, cervii18n.ErrorAgentCreateFailed, identity.Organization.ID, "", map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationDisplayNameRequired:       cervii18n.FieldAgentNameRequired,
			agentaction.ValidationTeamInvalid:               cervii18n.FieldMemberTeamInvalid,
			agentaction.ValidationModelInvalid:              cervii18n.FieldAgentModelInvalid,
			agentaction.ValidationSystemInstructionRequired: cervii18n.FieldAgentSystemInstructionRequired,
			agentaction.ValidationSystemInstructionTooLong:  cervii18n.FieldAgentSystemInstructionTooLong,
		})
	}
	slog.Info("AI 员工创建成功",
		"organization_id", identity.Organization.ID,
		"identity_id", created.IdentityID,
		"agent_id", created.ID,
		"revision_id", created.Capability.RevisionID,
		"provider_id", created.Capability.ProviderID,
		"model_identifier", created.Capability.ModelIdentifier,
	)
	return agentFromAction(*created), nil
}

// ListAgentModelOptions 返回企业 AI 员工可使用的对话模型。
func (b *DirectBackend) ListAgentModelOptions(ctx context.Context, meta RequestMeta) (AgentModelOptionList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return AgentModelOptionList{}, err
	}
	models, err := b.listAgentModelOptions.Execute(ctx, identity)
	if err != nil {
		return AgentModelOptionList{}, b.agentError(ctx, meta, err, cervii18n.ErrorAgentModelListFailed, identity.Organization.ID, "", nil)
	}
	output := make([]AgentModelOption, 0, len(models))
	for _, model := range models {
		output = append(output, AgentModelOption{
			ProviderID: model.ProviderID, ProviderName: model.ProviderName,
			ModelIdentifier: model.ModelIdentifier, ModelName: model.ModelName,
		})
	}
	return AgentModelOptionList{Models: output}, nil
}

// ListAgents 返回企业 AI 员工目录。
func (b *DirectBackend) ListAgents(ctx context.Context, meta RequestMeta, input AgentListInput) (AgentList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return AgentList{}, err
	}
	output, err := b.listAgents.Execute(ctx, identity, agentaction.ListInput{
		Query: input.Query, Status: optionalDomain[UserStatus, domain.UserStatus](input.Status), Page: input.Page, PageSize: input.PageSize,
	})
	if errors.Is(err, agentaction.ErrQueryInvalid) {
		return AgentList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	if err != nil {
		return AgentList{}, b.agentError(ctx, meta, err, cervii18n.ErrorAgentListFailed, identity.Organization.ID, "", nil)
	}
	agents := make([]AgentListItem, 0, len(output.Agents))
	for _, agent := range output.Agents {
		agents = append(agents, agentListItemFromAction(agent))
	}
	return AgentList{Agents: agents, Page: PageInfo{Number: output.Page, Size: output.Size, Total: output.Total}}, nil
}

// GetAgent 返回企业 AI 员工详情。
func (b *DirectBackend) GetAgent(ctx context.Context, meta RequestMeta, agentID string) (Agent, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Agent{}, err
	}
	agent, err := b.getAgent.Execute(ctx, identity, agentID)
	if err != nil {
		return Agent{}, b.agentError(ctx, meta, err, cervii18n.ErrorAgentReadFailed, identity.Organization.ID, agentID, nil)
	}
	return agentFromAction(*agent), nil
}

// UpdateAgent 修改企业 AI 员工名称和所属团队。
func (b *DirectBackend) UpdateAgent(ctx context.Context, meta RequestMeta, agentID string, input UpdateAgentInput) (Agent, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Agent{}, err
	}
	agent, err := b.updateAgent.Execute(ctx, identity, agentID, agentaction.UpdateInput{DisplayName: input.DisplayName, TeamIDs: input.TeamIDs})
	if err != nil {
		return Agent{}, b.agentError(ctx, meta, err, cervii18n.ErrorAgentUpdateFailed, identity.Organization.ID, agentID, map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationDisplayNameRequired: cervii18n.FieldAgentNameRequired,
			agentaction.ValidationTeamInvalid:         cervii18n.FieldMemberTeamInvalid,
		})
	}
	slog.Info("AI 员工已保存", "organization_id", identity.Organization.ID, "identity_id", agent.IdentityID, "agent_id", agentID)
	return agentFromAction(*agent), nil
}

// UpdateAgentCapability 修改企业 AI 员工的能力配置。
func (b *DirectBackend) UpdateAgentCapability(ctx context.Context, meta RequestMeta, agentID string, input AgentCapabilityInput) (Agent, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Agent{}, err
	}
	agent, err := b.updateAgentCapability.Execute(ctx, identity, agentID, agentCapabilityInput(input))
	if err != nil {
		return Agent{}, b.agentError(ctx, meta, err, cervii18n.ErrorAgentCapabilityUpdateFailed, identity.Organization.ID, agentID, map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationModelInvalid:              cervii18n.FieldAgentModelInvalid,
			agentaction.ValidationSystemInstructionRequired: cervii18n.FieldAgentSystemInstructionRequired,
			agentaction.ValidationSystemInstructionTooLong:  cervii18n.FieldAgentSystemInstructionTooLong,
		})
	}
	slog.Info("AI 员工能力配置已保存",
		"organization_id", identity.Organization.ID,
		"identity_id", agent.IdentityID,
		"agent_id", agentID,
		"revision_id", agent.Capability.RevisionID,
		"provider_id", agent.Capability.ProviderID,
		"model_identifier", agent.Capability.ModelIdentifier,
	)
	return agentFromAction(*agent), nil
}

// UpdateAgentWorkStatus 修改企业 AI 员工工作状态。
func (b *DirectBackend) UpdateAgentWorkStatus(ctx context.Context, meta RequestMeta, agentID string, input AgentWorkStatusInput) (Agent, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Agent{}, err
	}
	agent, err := b.updateAgentWorkStatus.Execute(ctx, identity, agentID, agentaction.WorkStatusInput{WorkStatus: domain.WorkStatus(input.WorkStatus)})
	if err != nil {
		return Agent{}, b.agentError(ctx, meta, err, cervii18n.ErrorWorkStatusUpdateFailed, identity.Organization.ID, agentID, map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationWorkStatusInvalid:     cervii18n.FieldWorkStatusInvalid,
			agentaction.ValidationWorkStatusUnavailable: cervii18n.FieldAgentWorkStatusUnavailable,
		})
	}
	slog.Info("AI 员工工作状态已修改", "organization_id", identity.Organization.ID, "identity_id", agent.IdentityID, "agent_id", agentID, "work_status", input.WorkStatus)
	return agentFromAction(*agent), nil
}

// DeactivateAgent 禁用企业 AI 员工账号。
func (b *DirectBackend) DeactivateAgent(ctx context.Context, meta RequestMeta, agentID string) (Agent, error) {
	return b.changeAgentStatus(ctx, meta, agentID, domain.UserStatusInactive)
}

// ReactivateAgent 恢复企业 AI 员工。
func (b *DirectBackend) ReactivateAgent(ctx context.Context, meta RequestMeta, agentID string) (Agent, error) {
	return b.changeAgentStatus(ctx, meta, agentID, domain.UserStatusActive)
}

// changeAgentStatus 修改企业 AI 员工账号状态。
func (b *DirectBackend) changeAgentStatus(ctx context.Context, meta RequestMeta, agentID string, status domain.UserStatus) (Agent, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Agent{}, err
	}
	agent, err := b.updateAgentStatus.Execute(ctx, identity, agentID, status)
	if err != nil {
		return Agent{}, b.agentError(ctx, meta, err, cervii18n.ErrorAgentStatusUpdateFailed, identity.Organization.ID, agentID, map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationStatusInvalid: cervii18n.FieldUserStatusInvalid,
		})
	}
	slog.Info("AI 员工账号状态已修改", "organization_id", identity.Organization.ID, "identity_id", agent.IdentityID, "agent_id", agentID, "status", status)
	return agentFromAction(*agent), nil
}

// agentFromAction 转换 AI 员工契约。
func agentFromAction(agent agentaction.Agent) Agent {
	teams := make([]TeamSummary, 0, len(agent.Teams))
	for _, team := range agent.Teams {
		teams = append(teams, TeamSummary{ID: team.ID, Name: team.Name})
	}
	capability := AgentCapability{
		ProviderID: agent.Capability.ProviderID, ProviderName: agent.Capability.ProviderName,
		ModelIdentifier: agent.Capability.ModelIdentifier, ModelName: agent.Capability.ModelName,
		SystemInstruction: agent.Capability.SystemInstruction,
	}
	return Agent{ID: agent.ID, IdentityID: agent.IdentityID, DisplayName: agent.DisplayName, Status: UserStatus(agent.Status), WorkStatus: WorkStatus(agent.WorkStatus), Teams: teams, Capability: capability, CreatedAt: agent.CreatedAt}
}

// agentListItemFromAction 转换 AI 员工目录项契约。
func agentListItemFromAction(agent agentaction.ListItem) AgentListItem {
	teams := make([]TeamSummary, 0, len(agent.Teams))
	for _, team := range agent.Teams {
		teams = append(teams, TeamSummary{ID: team.ID, Name: team.Name})
	}
	capability := AgentCapabilitySummary{
		ProviderID: agent.Capability.ProviderID, ProviderName: agent.Capability.ProviderName,
		ModelIdentifier: agent.Capability.ModelIdentifier, ModelName: agent.Capability.ModelName,
	}
	return AgentListItem{ID: agent.ID, IdentityID: agent.IdentityID, DisplayName: agent.DisplayName, Status: UserStatus(agent.Status), WorkStatus: WorkStatus(agent.WorkStatus), Teams: teams, Capability: capability, CreatedAt: agent.CreatedAt}
}

// agentCapabilityInput 转换 AI 员工能力配置输入。
func agentCapabilityInput(input AgentCapabilityInput) agentaction.CapabilityInput {
	return agentaction.CapabilityInput{
		ProviderID: input.ProviderID, ModelIdentifier: input.ModelIdentifier,
		SystemInstruction: input.SystemInstruction,
	}
}

// agentError 转换 AI 员工领域错误并记录未处理故障。
func (b *DirectBackend) agentError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, agentID string, fieldKeys map[common.FieldCode]cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, fieldKeys))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, agentaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorAgentNotFound)
	}
	attributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	if agentID != "" {
		attributes = append(attributes, "agent_id", agentID)
	}
	slog.Warn("AI 员工操作失败", attributes...)
	return FailedError(meta, failureKey)
}
