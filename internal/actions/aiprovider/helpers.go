//go:build server

package aiprovider

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// loadProvider 读取当前企业中的 AI 供应商。
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
		models = append(models, Model{
			Identifier: record.Identifier, Name: record.Name, ContextWindow: record.ContextWindow, MaxOutputTokens: record.MaxOutputTokens,
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
		records = append(records, servermodels.AIProviderModel{
			ProviderID: providerID, OrganizationID: organizationID, Identifier: model.Identifier,
			Name: model.Name, ContextWindow: model.ContextWindow, MaxOutputTokens: model.MaxOutputTokens,
		})
	}
	if len(records) == 0 {
		return nil
	}
	_, err := tx.NewInsert().
		Model(&records).
		Column("provider_id", "organization_id", "identifier", "name", "context_window", "max_output_tokens").
		Exec(ctx)
	return err
}

// recordFromModel 转换 AI 供应商存储模型。
func recordFromModel(provider servermodels.AIProvider, models []Model) Record {
	return Record{
		ID: provider.ID, Brand: domain.AIProviderBrand(provider.Brand), Name: provider.Name,
		APIKey: provider.APIKey, APIURL: provider.APIURL, Models: models,
	}
}

// isNameConflict 判断企业内供应商名称是否重复。
func isNameConflict(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) &&
		postgresError.Field('C') == "23505" &&
		postgresError.Field('n') == "ai_providers_organization_name_unique"
}
