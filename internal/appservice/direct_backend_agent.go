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
		DisplayName: input.DisplayName, RoleID: input.RoleID, TeamIDs: input.TeamIDs,
		Execution: agentExecutionInput(input.Execution),
	})
	if err != nil {
		return Agent{}, b.agentError(ctx, meta, err, cervii18n.ErrorAgentCreateFailed, identity.Organization.ID, "", map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationDisplayNameRequired:       cervii18n.FieldAgentNameRequired,
			agentaction.ValidationRoleInvalid:               cervii18n.FieldMemberRoleInvalid,
			agentaction.ValidationTeamInvalid:               cervii18n.FieldMemberTeamInvalid,
			agentaction.ValidationExecutionInvalid:          cervii18n.FieldAgentExecutionInvalid,
			agentaction.ValidationModelInvalid:              cervii18n.FieldAgentModelInvalid,
			agentaction.ValidationSystemInstructionRequired: cervii18n.FieldAgentSystemInstructionRequired,
			agentaction.ValidationSystemInstructionTooLong:  cervii18n.FieldAgentSystemInstructionTooLong,
		})
	}
	slog.Info("AI 员工创建成功",
		"organization_id", identity.Organization.ID,
		"identity_id", created.IdentityID,
		"agent_id", created.ID,
		"revision_id", created.Execution.RevisionID,
		"execution_mode", created.Execution.Mode,
		"provider_id", created.Execution.Managed.ProviderID,
		"model_identifier", created.Execution.Managed.ModelIdentifier,
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
	return AgentList{Agents: agents, Page: PageInfo{Number: output.Page.Number, Size: output.Page.Size, Total: output.Page.Total}}, nil
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
	agent, err := b.updateAgent.Execute(ctx, identity, agentID, agentaction.UpdateInput{DisplayName: input.DisplayName, RoleID: input.RoleID, TeamIDs: input.TeamIDs})
	if err != nil {
		return Agent{}, b.agentError(ctx, meta, err, cervii18n.ErrorAgentUpdateFailed, identity.Organization.ID, agentID, map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationDisplayNameRequired: cervii18n.FieldAgentNameRequired,
			agentaction.ValidationRoleInvalid:         cervii18n.FieldMemberRoleInvalid,
			agentaction.ValidationTeamInvalid:         cervii18n.FieldMemberTeamInvalid,
		})
	}
	slog.Info("AI 员工已保存", "organization_id", identity.Organization.ID, "identity_id", agent.IdentityID, "agent_id", agentID)
	return agentFromAction(*agent), nil
}

// UpdateAgentExecution 修改企业 AI 员工的执行配置。
func (b *DirectBackend) UpdateAgentExecution(ctx context.Context, meta RequestMeta, agentID string, input AgentExecutionInput) (Agent, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Agent{}, err
	}
	agent, err := b.updateAgentExecution.Execute(ctx, identity, agentID, agentExecutionInput(input))
	if err != nil {
		return Agent{}, b.agentError(ctx, meta, err, cervii18n.ErrorAgentExecutionUpdateFailed, identity.Organization.ID, agentID, map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationExecutionInvalid:          cervii18n.FieldAgentExecutionInvalid,
			agentaction.ValidationModelInvalid:              cervii18n.FieldAgentModelInvalid,
			agentaction.ValidationSystemInstructionRequired: cervii18n.FieldAgentSystemInstructionRequired,
			agentaction.ValidationSystemInstructionTooLong:  cervii18n.FieldAgentSystemInstructionTooLong,
		})
	}
	slog.Info("AI 员工执行配置已保存",
		"organization_id", identity.Organization.ID,
		"identity_id", agent.IdentityID,
		"agent_id", agentID,
		"revision_id", agent.Execution.RevisionID,
		"execution_mode", agent.Execution.Mode,
		"provider_id", agent.Execution.Managed.ProviderID,
		"model_identifier", agent.Execution.Managed.ModelIdentifier,
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
	return Agent{ID: agent.ID, IdentityID: agent.IdentityID, DisplayName: agent.DisplayName, Role: RoleSummary{ID: agent.RoleID, Kind: RoleKind(agent.RoleKind), Name: agent.RoleName}, Status: UserStatus(agent.Status), WorkStatus: WorkStatus(agent.WorkStatus), Teams: teams, Execution: agentExecutionFromAction(agent.Execution), CreatedAt: agent.CreatedAt}
}

// agentListItemFromAction 转换 AI 员工目录项契约。
func agentListItemFromAction(agent agentaction.ListItem) AgentListItem {
	teams := make([]TeamSummary, 0, len(agent.Teams))
	for _, team := range agent.Teams {
		teams = append(teams, TeamSummary{ID: team.ID, Name: team.Name})
	}
	return AgentListItem{ID: agent.ID, IdentityID: agent.IdentityID, DisplayName: agent.DisplayName, Role: RoleSummary{ID: agent.RoleID, Kind: RoleKind(agent.RoleKind), Name: agent.RoleName}, Status: UserStatus(agent.Status), WorkStatus: WorkStatus(agent.WorkStatus), Teams: teams, Execution: agentExecutionSummaryFromAction(agent.Execution), CreatedAt: agent.CreatedAt}
}

// agentExecutionInput 转换 AI 员工执行配置输入。
func agentExecutionInput(input AgentExecutionInput) agentaction.ExecutionInput {
	var managed *agentaction.ManagedExecutionInput
	if input.Managed != nil {
		managed = &agentaction.ManagedExecutionInput{
			ProviderID: input.Managed.ProviderID, ModelIdentifier: input.Managed.ModelIdentifier,
			SystemInstruction: input.Managed.SystemInstruction,
		}
	}
	return agentaction.ExecutionInput{Mode: domain.AgentExecutionMode(input.Mode), Managed: managed}
}

// agentExecutionFromAction 转换 AI 员工执行配置契约。
func agentExecutionFromAction(execution agentaction.Execution) AgentExecution {
	var managed *AgentManagedExecution
	if execution.Managed != nil {
		managed = &AgentManagedExecution{
			ProviderID: execution.Managed.ProviderID, ProviderName: execution.Managed.ProviderName,
			ModelIdentifier: execution.Managed.ModelIdentifier, ModelName: execution.Managed.ModelName,
			SystemInstruction: execution.Managed.SystemInstruction,
		}
	}
	return AgentExecution{RevisionID: execution.RevisionID, Mode: AgentExecutionMode(execution.Mode), Managed: managed}
}

// agentExecutionSummaryFromAction 转换 AI 员工执行配置摘要契约。
func agentExecutionSummaryFromAction(execution agentaction.ExecutionSummary) AgentExecutionSummary {
	var managed *AgentManagedExecutionSummary
	if execution.Managed != nil {
		managed = &AgentManagedExecutionSummary{
			ProviderID: execution.Managed.ProviderID, ProviderName: execution.Managed.ProviderName,
			ModelIdentifier: execution.Managed.ModelIdentifier, ModelName: execution.Managed.ModelName,
		}
	}
	return AgentExecutionSummary{RevisionID: execution.RevisionID, Mode: AgentExecutionMode(execution.Mode), Managed: managed}
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
