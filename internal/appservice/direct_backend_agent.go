//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// agentOps 持有 AI 员工与运行的 Action 和 Query。
type agentOps struct {
	agentCoordinator          *agentrunaction.ExecuteAction
	listAgentMCPServerOptions *agentaction.ListMCPServerOptionsQuery
	listAgentModelOptions     *agentaction.ListModelOptionsQuery
	createAgent               *agentaction.CreateAgentAction
	listAgents                *agentaction.ListAgentsQuery
	getAgent                  *agentaction.GetAgentQuery
	updateAgent               *agentaction.UpdateAgentAction
	updateAgentExecution      *agentaction.UpdateExecutionAction
	updateAgentStatus         *agentaction.UpdateStatusAction
}

// newAgentOps 创建 AI 员工与运行的业务实现依赖。
func newAgentOps(db *bun.DB, agentCoordinator *agentrunaction.ExecuteAction) agentOps {
	return agentOps{
		agentCoordinator:          agentCoordinator,
		listAgentMCPServerOptions: agentaction.NewListMCPServerOptionsQuery(db),
		listAgentModelOptions:     agentaction.NewListModelOptionsQuery(db),
		createAgent:               agentaction.NewCreateAgentAction(db),
		listAgents:                agentaction.NewListAgentsQuery(db),
		getAgent:                  agentaction.NewGetAgentQuery(db),
		updateAgent:               agentaction.NewUpdateAgentAction(db),
		updateAgentExecution:      agentaction.NewUpdateExecutionAction(db),
		updateAgentStatus:         agentaction.NewUpdateStatusAction(db),
	}
}

// CreateAgent 创建企业 AI 员工。
func (o *directOperations) CreateAgent(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input CreateAgentInput) (Agent, error) {
	created, err := o.createAgent.Execute(ctx, identity, agentaction.CreateInput{
		DisplayName: input.DisplayName, RoleID: input.RoleID, TeamIDs: input.TeamIDs,
		Execution: agentExecutionInput(input.Execution),
	})
	if err != nil {
		return Agent{}, o.agentError(ctx, meta, err, cervii18n.ErrorAgentCreateFailed, identity.Organization.ID, "", map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationDisplayNameRequired:       cervii18n.FieldAgentNameRequired,
			agentaction.ValidationRoleInvalid:               cervii18n.FieldMemberRoleInvalid,
			agentaction.ValidationTeamInvalid:               cervii18n.FieldTeamInvalid,
			agentaction.ValidationExecutionInvalid:          cervii18n.FieldAgentExecutionInvalid,
			agentaction.ValidationKnowledgeBaseInvalid:      cervii18n.FieldAgentKnowledgeBaseInvalid,
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
		"knowledge_base_count", len(created.Execution.Managed.KnowledgeBaseIDs),
	)
	return agentFromAction(*created), nil
}

// ListAgentMCPServerOptions 读取企业 MCP 服务摘要。
func (o *directOperations) ListAgentMCPServerOptions(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (AgentMCPServerOptionList, error) {
	options, err := o.listAgentMCPServerOptions.Execute(ctx, identity)
	if err != nil {
		return AgentMCPServerOptionList{}, o.agentError(ctx, meta, err, cervii18n.ErrorMCPServerListFailed, identity.Organization.ID, "", nil)
	}
	output := make([]AgentMCPServerOption, 0, len(options))
	for _, option := range options {
		output = append(output, AgentMCPServerOption{ID: option.ID, Name: option.Name, ToolCount: option.ToolCount})
	}
	return AgentMCPServerOptionList{MCPServers: output}, nil
}

// ListAgentModelOptions 返回企业 AI 员工可使用的对话模型。
func (o *directOperations) ListAgentModelOptions(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (AgentModelOptionList, error) {
	models, err := o.listAgentModelOptions.Execute(ctx, identity)
	if err != nil {
		return AgentModelOptionList{}, o.agentError(ctx, meta, err, cervii18n.ErrorAgentModelListFailed, identity.Organization.ID, "", nil)
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
func (o *directOperations) ListAgents(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input AgentListInput) (AgentList, error) {
	output, err := o.listAgents.Execute(ctx, identity, agentaction.ListInput{
		Query: input.Query, Status: optionalDomain[UserStatus, domain.UserStatus](input.Status), Page: input.Page, PageSize: input.PageSize,
	})
	if errors.Is(err, agentaction.ErrQueryInvalid) {
		return AgentList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	if err != nil {
		return AgentList{}, o.agentError(ctx, meta, err, cervii18n.ErrorAgentListFailed, identity.Organization.ID, "", nil)
	}
	agents := make([]AgentListItem, 0, len(output.Agents))
	for _, agent := range output.Agents {
		// 转换 AI 员工目录项契约。
		teams := make([]TeamSummary, 0, len(agent.Teams))
		for _, team := range agent.Teams {
			teams = append(teams, TeamSummary{ID: team.ID, Name: team.Name})
		}
		// 转换 AI 员工执行配置摘要契约。
		var managed *AgentManagedExecutionSummary
		if agent.Execution.Managed != nil {
			managed = &AgentManagedExecutionSummary{
				ProviderID: agent.Execution.Managed.ProviderID, ProviderName: agent.Execution.Managed.ProviderName,
				ModelIdentifier: agent.Execution.Managed.ModelIdentifier, ModelName: agent.Execution.Managed.ModelName,
			}
		}
		execution := AgentExecutionSummary{RevisionID: agent.Execution.RevisionID, Mode: AgentExecutionMode(agent.Execution.Mode), Managed: managed}
		agents = append(agents, AgentListItem{ID: agent.ID, IdentityID: agent.IdentityID, DisplayName: agent.DisplayName, Role: RoleSummary{ID: agent.RoleID, Kind: RoleKind(agent.RoleKind), Name: agent.RoleName}, Status: UserStatus(agent.Status), WorkStatus: WorkStatus(agent.WorkStatus), Teams: teams, Execution: execution, CreatedAt: agent.CreatedAt})
	}
	return AgentList{Agents: agents, Page: PageInfo{Number: output.Page.Number, Size: output.Page.Size, Total: output.Page.Total}}, nil
}

// GetAgent 返回企业 AI 员工详情。
func (o *directOperations) GetAgent(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, agentID string) (Agent, error) {
	agent, err := o.getAgent.Execute(ctx, identity, agentID)
	if err != nil {
		return Agent{}, o.agentError(ctx, meta, err, cervii18n.ErrorAgentReadFailed, identity.Organization.ID, agentID, nil)
	}
	return agentFromAction(*agent), nil
}

// UpdateAgent 保存企业 AI 员工基本资料和工作状态。
func (o *directOperations) UpdateAgent(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, agentID string, input UpdateAgentInput) (Agent, error) {
	agent, err := o.updateAgent.Execute(ctx, identity, agentID, agentaction.UpdateInput{DisplayName: input.DisplayName, RoleID: input.RoleID, TeamIDs: input.TeamIDs, WorkStatus: domain.WorkStatus(input.WorkStatus)})
	if err != nil {
		return Agent{}, o.agentError(ctx, meta, err, cervii18n.ErrorAgentUpdateFailed, identity.Organization.ID, agentID, map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationDisplayNameRequired:   cervii18n.FieldAgentNameRequired,
			agentaction.ValidationRoleInvalid:           cervii18n.FieldMemberRoleInvalid,
			agentaction.ValidationTeamInvalid:           cervii18n.FieldTeamInvalid,
			agentaction.ValidationWorkStatusInvalid:     cervii18n.FieldWorkStatusInvalid,
			agentaction.ValidationWorkStatusUnavailable: cervii18n.FieldAgentWorkStatusUnavailable,
		})
	}
	slog.Info("AI 员工已保存", "organization_id", identity.Organization.ID, "identity_id", agent.IdentityID, "agent_id", agentID, "work_status", agent.WorkStatus)
	return agentFromAction(*agent), nil
}

// UpdateAgentExecution 修改企业 AI 员工的执行配置。
func (o *directOperations) UpdateAgentExecution(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, agentID string, input UpdateAgentExecutionInput) (Agent, error) {
	agent, err := o.updateAgentExecution.Execute(ctx, identity, agentID, agentaction.UpdateExecutionInput{
		ExecutionInput: agentExecutionInput(AgentExecutionInput{Mode: input.Mode, Managed: input.Managed}),
		MCPServerIDs:   input.MCPServerIDs,
	})
	if err != nil {
		return Agent{}, o.agentError(ctx, meta, err, cervii18n.ErrorAgentExecutionUpdateFailed, identity.Organization.ID, agentID, map[common.FieldCode]cervii18n.Key{
			agentaction.ValidationMCPServerInvalid:          cervii18n.FieldAgentMCPServerInvalid,
			agentaction.ValidationExecutionInvalid:          cervii18n.FieldAgentExecutionInvalid,
			agentaction.ValidationKnowledgeBaseInvalid:      cervii18n.FieldAgentKnowledgeBaseInvalid,
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
		"knowledge_base_count", len(agent.Execution.Managed.KnowledgeBaseIDs),
		"mcp_server_count", len(agent.Execution.MCPServerIDs),
	)
	return agentFromAction(*agent), nil
}

// DeactivateAgent 禁用企业 AI 员工账号。
func (o *directOperations) DeactivateAgent(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, agentID string) (Agent, error) {
	return o.changeAgentStatus(ctx, meta, identity, agentID, domain.UserStatusInactive)
}

// ReactivateAgent 恢复企业 AI 员工。
func (o *directOperations) ReactivateAgent(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, agentID string) (Agent, error) {
	return o.changeAgentStatus(ctx, meta, identity, agentID, domain.UserStatusActive)
}

// changeAgentStatus 修改企业 AI 员工账号状态。
func (o *directOperations) changeAgentStatus(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, agentID string, status domain.UserStatus) (Agent, error) {
	agent, err := o.updateAgentStatus.Execute(ctx, identity, agentID, status)
	if err != nil {
		return Agent{}, o.agentError(ctx, meta, err, cervii18n.ErrorAgentStatusUpdateFailed, identity.Organization.ID, agentID, map[common.FieldCode]cervii18n.Key{
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
	// 转换 AI 员工执行配置契约。
	var managed *AgentManagedExecution
	if agent.Execution.Managed != nil {
		managed = &AgentManagedExecution{
			ProviderID: agent.Execution.Managed.ProviderID, ProviderName: agent.Execution.Managed.ProviderName,
			ModelIdentifier: agent.Execution.Managed.ModelIdentifier, ModelName: agent.Execution.Managed.ModelName,
			SystemInstruction: agent.Execution.Managed.SystemInstruction,
			KnowledgeBaseIDs:  agent.Execution.Managed.KnowledgeBaseIDs,
		}
	}
	execution := AgentExecution{MCPServerIDs: agent.Execution.MCPServerIDs, RevisionID: agent.Execution.RevisionID, Mode: AgentExecutionMode(agent.Execution.Mode), Managed: managed}
	return Agent{ID: agent.ID, IdentityID: agent.IdentityID, DisplayName: agent.DisplayName, Role: RoleSummary{ID: agent.RoleID, Kind: RoleKind(agent.RoleKind), Name: agent.RoleName}, Status: UserStatus(agent.Status), WorkStatus: WorkStatus(agent.WorkStatus), Teams: teams, Execution: execution, CreatedAt: agent.CreatedAt}
}

// agentExecutionInput 转换 AI 员工执行配置输入。
func agentExecutionInput(input AgentExecutionInput) agentaction.ExecutionInput {
	var managed *agentaction.ManagedExecutionInput
	if input.Managed != nil {
		managed = &agentaction.ManagedExecutionInput{
			ProviderID: input.Managed.ProviderID, ModelIdentifier: input.Managed.ModelIdentifier,
			SystemInstruction: input.Managed.SystemInstruction,
			KnowledgeBaseIDs:  input.Managed.KnowledgeBaseIDs,
		}
	}
	return agentaction.ExecutionInput{Mode: domain.AgentExecutionMode(input.Mode), Managed: managed}
}

// agentError 转换 AI 员工领域错误并记录未处理故障。
func (o *directOperations) agentError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, agentID string, fieldKeys map[common.FieldCode]cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
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
