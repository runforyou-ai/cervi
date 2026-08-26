//go:build server

package agent

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const (
	managedExecutionSchemaVersion = 1
	maxSystemInstructionLength    = 20000
)

// ExecutionInput 定义 AI 员工执行配置输入。
type ExecutionInput struct {
	Mode    domain.AgentExecutionMode
	Managed *ManagedExecutionInput
}

// ManagedExecutionInput 定义平台托管执行配置输入。
type ManagedExecutionInput struct {
	ProviderID        string
	ModelIdentifier   string
	SystemInstruction string
}

// Execution 定义 AI 员工当前生效的执行配置。
type Execution struct {
	RevisionID string
	Mode       domain.AgentExecutionMode
	Managed    *ManagedExecution
}

// ManagedExecution 定义平台托管执行配置。
type ManagedExecution struct {
	ProviderID        string
	ProviderName      string
	ModelIdentifier   string
	ModelName         string
	SystemInstruction string
}

// ExecutionSummary 定义 AI 员工当前执行配置摘要。
type ExecutionSummary struct {
	RevisionID string
	Mode       domain.AgentExecutionMode
	Managed    *ManagedExecutionSummary
}

// ManagedExecutionSummary 定义平台托管执行配置摘要。
type ManagedExecutionSummary struct {
	ProviderID      string
	ProviderName    string
	ModelIdentifier string
	ModelName       string
}

// ModelOption 定义 AI 员工可用的对话模型。
type ModelOption struct {
	ProviderID      string `bun:"provider_id"`
	ProviderName    string `bun:"provider_name"`
	ModelIdentifier string `bun:"model_identifier"`
	ModelName       string `bun:"model_name"`
}

type managedRevisionConfigurationV1 struct {
	Model             managedRevisionModelV1 `json:"model"`
	SystemInstruction string                 `json:"systemInstruction"`
}

type managedRevisionModelV1 struct {
	ProviderID   string `json:"providerId"`
	ProviderName string `json:"providerName"`
	Identifier   string `json:"identifier"`
	Name         string `json:"name"`
}

// normalizeExecutionInput 规范化并校验 AI 员工执行配置。
func normalizeExecutionInput(input ExecutionInput) (ExecutionInput, error) {
	if input.Mode != domain.AgentExecutionModeManaged || input.Managed == nil {
		return ExecutionInput{}, &common.FieldError{Fields: map[string]common.FieldCode{"execution": ValidationExecutionInvalid}}
	}
	managed, err := normalizeManagedExecutionInput(*input.Managed)
	if err != nil {
		return ExecutionInput{}, err
	}
	input.Managed = &managed
	return input, nil
}

// normalizeManagedExecutionInput 规范化并校验平台托管执行配置。
func normalizeManagedExecutionInput(input ManagedExecutionInput) (ManagedExecutionInput, error) {
	var providerIDValid bool
	input.ProviderID, providerIDValid = common.NormalizeUUID(input.ProviderID)
	input.ModelIdentifier = strings.TrimSpace(input.ModelIdentifier)
	input.SystemInstruction = strings.TrimSpace(input.SystemInstruction)
	fields := make(map[string]common.FieldCode)
	if !providerIDValid {
		fields["providerId"] = ValidationModelInvalid
	}
	if input.ModelIdentifier == "" {
		fields["modelIdentifier"] = ValidationModelInvalid
	}
	if input.SystemInstruction == "" {
		fields["systemInstruction"] = ValidationSystemInstructionRequired
	} else if utf8.RuneCountInString(input.SystemInstruction) > maxSystemInstructionLength {
		fields["systemInstruction"] = ValidationSystemInstructionTooLong
	}
	if len(fields) > 0 {
		return ManagedExecutionInput{}, &common.FieldError{Fields: fields}
	}
	return input, nil
}

// loadManagedExecutionModel 校验平台托管执行配置使用的文本对话模型。
func loadManagedExecutionModel(ctx context.Context, db bun.IDB, organizationID string, input ManagedExecutionInput) (ModelOption, error) {
	model := ModelOption{}
	if err := managedExecutionModelQuery(db, organizationID, input.ProviderID, input.ModelIdentifier).
		For("KEY SHARE OF aip, aipm").
		Scan(ctx, &model); errors.Is(err, sql.ErrNoRows) {
		return ModelOption{}, &common.FieldError{Fields: map[string]common.FieldCode{"modelIdentifier": ValidationModelInvalid}}
	} else if err != nil {
		return ModelOption{}, err
	}
	return model, nil
}

// loadStoredManagedExecutionModel 读取已保存平台托管配置当前对应的模型目录项。
func loadStoredManagedExecutionModel(ctx context.Context, db bun.IDB, organizationID, providerID, modelIdentifier string) (ModelOption, error) {
	model := ModelOption{}
	if err := managedExecutionModelQuery(db, organizationID, providerID, modelIdentifier).Scan(ctx, &model); errors.Is(err, sql.ErrNoRows) {
		return ModelOption{}, fmt.Errorf("managed execution model %q/%q is unavailable", providerID, modelIdentifier)
	} else if err != nil {
		return ModelOption{}, err
	}
	return model, nil
}

// managedExecutionModelQuery 构造平台托管执行配置的模型目录查询。
func managedExecutionModelQuery(db bun.IDB, organizationID, providerID, modelIdentifier string) *bun.SelectQuery {
	return db.NewSelect().TableExpr("ai_provider_models AS aipm").
		ColumnExpr("aip.id::text AS provider_id, aip.name AS provider_name, aipm.identifier AS model_identifier, aipm.name AS model_name").
		Join("JOIN ai_providers AS aip ON aip.id = aipm.provider_id AND aip.organization_id = aipm.organization_id").
		Where("aipm.organization_id = ?", organizationID).
		Where("aipm.provider_id = ?", providerID).
		Where("aipm.identifier = ?", modelIdentifier).
		Where("aipm.model_type = ?", domain.AIModelTypeChat).
		Where("aipm.input_modalities @> ?::jsonb", `["text"]`)
}

// insertExecutionRevision 创建 AI 员工执行配置版本。
func insertExecutionRevision(ctx context.Context, db bun.IDB, identity *servermodels.Identity, agentID, revisionID string, input ExecutionInput, model ModelOption) (Execution, error) {
	configuration, err := json.Marshal(managedRevisionConfigurationV1{
		Model: managedRevisionModelV1{
			ProviderID: model.ProviderID, ProviderName: model.ProviderName,
			Identifier: model.ModelIdentifier, Name: model.ModelName,
		},
		SystemInstruction: input.Managed.SystemInstruction,
	})
	if err != nil {
		return Execution{}, err
	}
	revision := &servermodels.AgentRevision{
		ID: revisionID, OrganizationID: identity.Organization.ID, AgentID: agentID,
		ExecutionMode: string(input.Mode), SchemaVersion: managedExecutionSchemaVersion,
		Configuration: configuration, CreatedByUserID: identity.User.ID,
	}
	if _, err := db.NewInsert().Model(revision).
		Column("id", "organization_id", "agent_id", "execution_mode", "schema_version", "configuration", "created_by_user_id").
		Exec(ctx); err != nil {
		return Execution{}, err
	}
	return Execution{
		RevisionID: revision.ID, Mode: input.Mode,
		Managed: &ManagedExecution{
			ProviderID: model.ProviderID, ProviderName: model.ProviderName,
			ModelIdentifier: model.ModelIdentifier, ModelName: model.ModelName,
			SystemInstruction: input.Managed.SystemInstruction,
		},
	}, nil
}

// decodeRevisionExecution 解码不可变执行配置版本。
func decodeRevisionExecution(revision servermodels.AgentRevision) (Execution, error) {
	mode := domain.AgentExecutionMode(revision.ExecutionMode)
	if mode != domain.AgentExecutionModeManaged {
		return Execution{}, fmt.Errorf("unsupported agent execution mode %q", revision.ExecutionMode)
	}
	if revision.SchemaVersion != managedExecutionSchemaVersion {
		return Execution{}, fmt.Errorf("unsupported managed execution schema version %d", revision.SchemaVersion)
	}
	configuration := managedRevisionConfigurationV1{}
	decoder := json.NewDecoder(bytes.NewReader(revision.Configuration))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&configuration); err != nil {
		return Execution{}, fmt.Errorf("decode managed execution configuration: %w", err)
	}
	if !common.ValidUUID(configuration.Model.ProviderID) ||
		strings.TrimSpace(configuration.Model.ProviderName) == "" ||
		strings.TrimSpace(configuration.Model.Identifier) == "" ||
		strings.TrimSpace(configuration.Model.Name) == "" ||
		strings.TrimSpace(configuration.SystemInstruction) == "" ||
		utf8.RuneCountInString(configuration.SystemInstruction) > maxSystemInstructionLength {
		return Execution{}, errors.New("managed execution configuration is invalid")
	}
	return Execution{
		RevisionID: revision.ID, Mode: mode,
		Managed: &ManagedExecution{
			ProviderID: configuration.Model.ProviderID, ProviderName: configuration.Model.ProviderName,
			ModelIdentifier: configuration.Model.Identifier, ModelName: configuration.Model.Name,
			SystemInstruction: configuration.SystemInstruction,
		},
	}, nil
}

// loadAgentExecution 读取 AI 员工当前生效的执行配置。
func loadAgentExecution(ctx context.Context, db bun.IDB, organizationID, agentID string) (Execution, error) {
	revision := &servermodels.AgentRevision{}
	if err := db.NewSelect().Model(revision).
		Join("JOIN agents AS a ON a.active_revision_id = ar.id AND a.organization_id = ar.organization_id AND a.id = ar.agent_id").
		Where("a.organization_id = ?", organizationID).
		Where("a.id = ?", agentID).
		Scan(ctx); err != nil {
		return Execution{}, err
	}
	execution, err := decodeRevisionExecution(*revision)
	if err != nil {
		return Execution{}, err
	}
	model, err := loadStoredManagedExecutionModel(ctx, db, organizationID, execution.Managed.ProviderID, execution.Managed.ModelIdentifier)
	if err != nil {
		return Execution{}, err
	}
	execution.Managed.ProviderName = model.ProviderName
	execution.Managed.ModelName = model.ModelName
	return execution, nil
}

// loadAgentExecutionSummaries 批量读取 AI 员工当前执行配置摘要。
func loadAgentExecutionSummaries(ctx context.Context, db bun.IDB, organizationID string, agentIDs []string) (map[string]ExecutionSummary, error) {
	type row struct {
		AgentID       string          `bun:"agent_id"`
		RevisionID    string          `bun:"revision_id"`
		ExecutionMode string          `bun:"execution_mode"`
		SchemaVersion int             `bun:"schema_version"`
		Configuration json.RawMessage `bun:"configuration"`
	}
	if len(agentIDs) == 0 {
		return map[string]ExecutionSummary{}, nil
	}
	records := make([]row, 0, len(agentIDs))
	if err := db.NewSelect().TableExpr("agents AS a").
		ColumnExpr("a.id::text AS agent_id, ar.id::text AS revision_id, ar.execution_mode, ar.schema_version, ar.configuration").
		Join("JOIN agent_revisions AS ar ON ar.id = a.active_revision_id AND ar.organization_id = a.organization_id AND ar.agent_id = a.id").
		Where("a.organization_id = ?", organizationID).
		Where("a.id IN (?)", bun.In(agentIDs)).
		Scan(ctx, &records); err != nil {
		return nil, err
	}
	if len(records) != len(agentIDs) {
		return nil, fmt.Errorf("agent execution revision count %d does not match agent count %d", len(records), len(agentIDs))
	}
	executions := make(map[string]Execution, len(records))
	providerIDs := make(map[string]struct{})
	for _, record := range records {
		execution, err := decodeRevisionExecution(servermodels.AgentRevision{
			ID: record.RevisionID, ExecutionMode: record.ExecutionMode,
			SchemaVersion: record.SchemaVersion, Configuration: record.Configuration,
		})
		if err != nil {
			return nil, fmt.Errorf("decode agent %q execution: %w", record.AgentID, err)
		}
		executions[record.AgentID] = execution
		providerIDs[execution.Managed.ProviderID] = struct{}{}
	}
	providerIDList := make([]string, 0, len(providerIDs))
	for providerID := range providerIDs {
		providerIDList = append(providerIDList, providerID)
	}
	models := make([]ModelOption, 0)
	if err := db.NewSelect().TableExpr("ai_provider_models AS aipm").
		ColumnExpr("aip.id::text AS provider_id, aip.name AS provider_name, aipm.identifier AS model_identifier, aipm.name AS model_name").
		Join("JOIN ai_providers AS aip ON aip.id = aipm.provider_id AND aip.organization_id = aipm.organization_id").
		Where("aipm.organization_id = ?", organizationID).
		Where("aipm.provider_id IN (?)", bun.In(providerIDList)).
		Where("aipm.model_type = ?", domain.AIModelTypeChat).
		Where("aipm.input_modalities @> ?::jsonb", `["text"]`).
		Scan(ctx, &models); err != nil {
		return nil, err
	}
	modelByKey := make(map[string]ModelOption, len(models))
	for _, model := range models {
		modelByKey[executionModelKey(model.ProviderID, model.ModelIdentifier)] = model
	}
	summaries := make(map[string]ExecutionSummary, len(executions))
	for agentID, execution := range executions {
		model, exists := modelByKey[executionModelKey(execution.Managed.ProviderID, execution.Managed.ModelIdentifier)]
		if !exists {
			return nil, fmt.Errorf("agent %q managed execution model is unavailable", agentID)
		}
		summaries[agentID] = ExecutionSummary{
			RevisionID: execution.RevisionID, Mode: execution.Mode,
			Managed: &ManagedExecutionSummary{
				ProviderID: model.ProviderID, ProviderName: model.ProviderName,
				ModelIdentifier: model.ModelIdentifier, ModelName: model.ModelName,
			},
		}
	}
	return summaries, nil
}

// executionModelKey 返回模型服务和模型标识的组合键。
func executionModelKey(providerID, modelIdentifier string) string {
	return providerID + "\x00" + modelIdentifier
}
