//go:build server

package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const maxSystemInstructionLength = 20000

// CapabilityInput 定义 AI 员工能力配置输入。
type CapabilityInput struct {
	ProviderID        string
	ModelIdentifier   string
	SystemInstruction string
}

// Capability 定义 AI 员工当前生效的能力配置。
type Capability struct {
	RevisionID        string `bun:"revision_id"`
	ProviderID        string `bun:"provider_id"`
	ProviderName      string `bun:"provider_name"`
	ModelIdentifier   string `bun:"model_identifier"`
	ModelName         string `bun:"model_name"`
	SystemInstruction string `bun:"system_instruction"`
}

// CapabilitySummary 定义 AI 员工当前模型摘要。
type CapabilitySummary struct {
	ProviderID      string `bun:"provider_id"`
	ProviderName    string `bun:"provider_name"`
	ModelIdentifier string `bun:"model_identifier"`
	ModelName       string `bun:"model_name"`
}

// ModelOption 定义 AI 员工可用的对话模型。
type ModelOption struct {
	ProviderID      string `bun:"provider_id"`
	ProviderName    string `bun:"provider_name"`
	ModelIdentifier string `bun:"model_identifier"`
	ModelName       string `bun:"model_name"`
}

// normalizeCapabilityInput 规范化并校验 AI 员工能力配置。
func normalizeCapabilityInput(input CapabilityInput) (CapabilityInput, error) {
	input.ProviderID = strings.TrimSpace(input.ProviderID)
	input.ModelIdentifier = strings.TrimSpace(input.ModelIdentifier)
	input.SystemInstruction = strings.TrimSpace(input.SystemInstruction)
	fields := make(map[string]common.FieldCode)
	if !common.ValidUUID(input.ProviderID) {
		fields["providerId"] = ValidationModelInvalid
	}
	if input.ModelIdentifier == "" {
		fields["modelIdentifier"] = ValidationModelInvalid
	}
	if input.SystemInstruction == "" {
		fields["systemInstruction"] = ValidationSystemInstructionRequired
	} else if len([]rune(input.SystemInstruction)) > maxSystemInstructionLength {
		fields["systemInstruction"] = ValidationSystemInstructionTooLong
	}
	if len(fields) > 0 {
		return CapabilityInput{}, &common.FieldError{Fields: fields}
	}
	return input, nil
}

// loadCapabilityModel 校验文本对话模型，并通过共享键锁与供应商写操作串行。
func loadCapabilityModel(ctx context.Context, db bun.IDB, organizationID string, input CapabilityInput) (ModelOption, error) {
	model := ModelOption{}
	if err := db.NewSelect().TableExpr("ai_provider_models AS aipm").
		ColumnExpr("aip.id::text AS provider_id, aip.name AS provider_name, aipm.identifier AS model_identifier, aipm.name AS model_name").
		Join("JOIN ai_providers AS aip ON aip.id = aipm.provider_id AND aip.organization_id = aipm.organization_id").
		Where("aipm.organization_id = ?", organizationID).
		Where("aipm.provider_id = ?", input.ProviderID).
		Where("aipm.identifier = ?", input.ModelIdentifier).
		Where("aipm.model_type = ?", domain.AIModelTypeChat).
		Where("aipm.input_modalities @> ?::jsonb", `["text"]`).
		For("KEY SHARE OF aip, aipm").
		Scan(ctx, &model); errors.Is(err, sql.ErrNoRows) {
		return ModelOption{}, &common.FieldError{Fields: map[string]common.FieldCode{"modelIdentifier": ValidationModelInvalid}}
	} else if err != nil {
		return ModelOption{}, err
	}
	return model, nil
}

// insertCapabilityRevision 创建 AI 员工配置版本。
func insertCapabilityRevision(ctx context.Context, db bun.IDB, identity *servermodels.Identity, agentID, revisionID string, input CapabilityInput, model ModelOption) (Capability, error) {
	revision := &servermodels.AgentRevision{
		ID:             revisionID,
		OrganizationID: identity.Organization.ID, AgentID: agentID,
		ProviderID: model.ProviderID, ModelIdentifier: model.ModelIdentifier,
		SystemInstruction: input.SystemInstruction, CreatedByUserID: identity.User.ID,
	}
	if _, err := db.NewInsert().Model(revision).
		Column("id", "organization_id", "agent_id", "provider_id", "model_identifier", "system_instruction", "created_by_user_id").
		Exec(ctx); err != nil {
		return Capability{}, err
	}
	return Capability{
		RevisionID: revision.ID, ProviderID: model.ProviderID, ProviderName: model.ProviderName,
		ModelIdentifier: model.ModelIdentifier, ModelName: model.ModelName,
		SystemInstruction: input.SystemInstruction,
	}, nil
}

// loadAgentCapability 读取 AI 员工当前生效的能力配置。
func loadAgentCapability(ctx context.Context, db bun.IDB, organizationID, agentID string) (Capability, error) {
	capability := Capability{}
	err := db.NewSelect().TableExpr("agents AS a").
		ColumnExpr("ar.id::text AS revision_id, ar.provider_id::text AS provider_id, aip.name AS provider_name, ar.model_identifier, aipm.name AS model_name, ar.system_instruction").
		Join("JOIN agent_revisions AS ar ON ar.id = a.active_revision_id AND ar.organization_id = a.organization_id AND ar.agent_id = a.id").
		Join("JOIN ai_providers AS aip ON aip.id = ar.provider_id AND aip.organization_id = ar.organization_id").
		Join("JOIN ai_provider_models AS aipm ON aipm.provider_id = ar.provider_id AND aipm.organization_id = ar.organization_id AND aipm.identifier = ar.model_identifier").
		Where("a.organization_id = ?", organizationID).
		Where("a.id = ?", agentID).
		Scan(ctx, &capability)
	return capability, err
}

// loadAgentCapabilitySummaries 批量读取 AI 员工当前模型摘要。
func loadAgentCapabilitySummaries(ctx context.Context, db bun.IDB, organizationID string, agentIDs []string) (map[string]CapabilitySummary, error) {
	type row struct {
		AgentID         string `bun:"agent_id"`
		ProviderID      string `bun:"provider_id"`
		ProviderName    string `bun:"provider_name"`
		ModelIdentifier string `bun:"model_identifier"`
		ModelName       string `bun:"model_name"`
	}
	if len(agentIDs) == 0 {
		return map[string]CapabilitySummary{}, nil
	}
	records := make([]row, 0, len(agentIDs))
	if err := db.NewSelect().TableExpr("agents AS a").
		ColumnExpr("a.id::text AS agent_id, ar.provider_id::text AS provider_id, aip.name AS provider_name, ar.model_identifier, aipm.name AS model_name").
		Join("JOIN agent_revisions AS ar ON ar.id = a.active_revision_id AND ar.organization_id = a.organization_id AND ar.agent_id = a.id").
		Join("JOIN ai_providers AS aip ON aip.id = ar.provider_id AND aip.organization_id = ar.organization_id").
		Join("JOIN ai_provider_models AS aipm ON aipm.provider_id = ar.provider_id AND aipm.organization_id = ar.organization_id AND aipm.identifier = ar.model_identifier").
		Where("a.organization_id = ?", organizationID).
		Where("a.id IN (?)", bun.In(agentIDs)).
		Scan(ctx, &records); err != nil {
		return nil, err
	}
	capabilities := make(map[string]CapabilitySummary, len(records))
	for _, record := range records {
		capabilities[record.AgentID] = CapabilitySummary{
			ProviderID: record.ProviderID, ProviderName: record.ProviderName,
			ModelIdentifier: record.ModelIdentifier, ModelName: record.ModelName,
		}
	}
	if len(capabilities) != len(agentIDs) {
		return nil, fmt.Errorf("agent capability summary count %d does not match agent count %d", len(capabilities), len(agentIDs))
	}
	return capabilities, nil
}
