//go:build server

package aiprovider

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

// loadProvider 读取当前企业中的模型服务供应商。
func loadProvider(ctx context.Context, db bun.IDB, organizationID, providerID string, lock bool) (*servermodels.AIProvider, error) {
	if !common.ValidUUID(providerID) {
		return nil, ErrNotFound
	}
	provider := &servermodels.AIProvider{}
	query := db.NewSelect().
		Model(provider).
		Where("aip.id = ?", providerID).
		Where("aip.organization_id = ?", organizationID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return provider, nil
}

// loadModels 读取供应商的模型目录。
func loadModels(ctx context.Context, db bun.IDB, organizationID, providerID string) ([]Model, error) {
	records := make([]servermodels.AIProviderModel, 0)
	if err := db.NewSelect().
		Model(&records).
		Where("aipm.organization_id = ?", organizationID).
		Where("aipm.provider_id = ?", providerID).
		Order("aipm.created_at ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	models := make([]Model, 0, len(records))
	for _, record := range records {
		inputModalities := make([]domain.AIModelInputModality, 0)
		if err := json.Unmarshal(record.InputModalities, &inputModalities); err != nil {
			return nil, fmt.Errorf("decode model %q input modalities: %w", record.Identifier, err)
		}
		models = append(models, Model{
			Identifier: record.Identifier, Name: record.Name, Type: domain.AIModelType(record.Type),
			InputModalities: inputModalities, ContextWindow: record.ContextWindow, MaxOutputTokens: record.MaxOutputTokens,
		})
	}
	return models, nil
}

// replaceModels 替换供应商的全部已启用模型。
func replaceModels(ctx context.Context, tx bun.Tx, organizationID, providerID string, models []Model) error {
	if _, err := tx.NewDelete().
		Model((*servermodels.AIProviderModel)(nil)).
		Where("organization_id = ?", organizationID).
		Where("provider_id = ?", providerID).
		Exec(ctx); err != nil {
		return err
	}
	records := make([]servermodels.AIProviderModel, 0, len(models))
	for _, model := range models {
		inputModalities, err := json.Marshal(model.InputModalities)
		if err != nil {
			return err
		}
		records = append(records, servermodels.AIProviderModel{
			ProviderID: providerID, OrganizationID: organizationID, Identifier: model.Identifier,
			Name: model.Name, Type: string(model.Type), ContextWindow: model.ContextWindow, MaxOutputTokens: model.MaxOutputTokens,
			InputModalities: inputModalities,
		})
	}
	_, err := tx.NewInsert().
		Model(&records).
		Column(
			"provider_id", "organization_id", "identifier", "name", "model_type", "input_modalities",
			"context_window", "max_output_tokens",
		).
		Exec(ctx)
	return err
}

// validateReferencedModels 校验新目录保留 AI 员工正在使用的文本对话模型。
func validateReferencedModels(ctx context.Context, db bun.IDB, organizationID, providerID string, models []Model) error {
	activeIdentifiers := make([]string, 0)
	if err := db.NewSelect().TableExpr("agents AS a").
		ColumnExpr("DISTINCT ar.configuration #>> '{model,identifier}'").
		Join("JOIN agent_revisions AS ar ON ar.id = a.active_revision_id AND ar.organization_id = a.organization_id AND ar.agent_id = a.id").
		Where("a.organization_id = ?", organizationID).
		Where("ar.execution_mode = ?", domain.AgentExecutionModeManaged).
		Where("ar.configuration #>> '{model,providerId}' = ?", providerID).
		Scan(ctx, &activeIdentifiers); err != nil {
		return err
	}
	available := make(map[string]struct{}, len(models))
	for _, model := range models {
		if model.Type == domain.AIModelTypeChat && modelSupportsText(model) {
			available[model.Identifier] = struct{}{}
		}
	}
	for _, identifier := range activeIdentifiers {
		if _, exists := available[identifier]; !exists {
			return &ValidationError{Fields: map[string]ValidationCode{"models": ValidationModelsInUse}}
		}
	}
	return nil
}

// modelSupportsText 判断模型是否支持文本输入。
func modelSupportsText(model Model) bool {
	for _, modality := range model.InputModalities {
		if modality == domain.AIModelInputModalityText {
			return true
		}
	}
	return false
}

// providerInUse 判断供应商是否被 AI 员工使用。
func providerInUse(ctx context.Context, db bun.IDB, organizationID, providerID string) (bool, error) {
	return db.NewSelect().TableExpr("agents AS a").
		Join("JOIN agent_revisions AS ar ON ar.id = a.active_revision_id AND ar.organization_id = a.organization_id AND ar.agent_id = a.id").
		Where("a.organization_id = ?", organizationID).
		Where("ar.execution_mode = ?", domain.AgentExecutionModeManaged).
		Where("ar.configuration #>> '{model,providerId}' = ?", providerID).
		Exists(ctx)
}

// recordFromModel 转换模型服务供应商存储模型。
func recordFromModel(provider servermodels.AIProvider, models []Model) Record {
	return Record{
		ID: provider.ID, Brand: domain.AIProviderBrand(provider.Brand), Name: provider.Name,
		APIKey: provider.APIKey, APIURL: provider.APIURL, Models: models,
	}
}

// isNameConflict 判断企业内供应商名称是否重复。
func isNameConflict(err error) bool {
	return pgerr.UniqueViolationOn(err, "ai_providers_organization_name_unique")
}
