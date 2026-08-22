//go:build server

package aiprovider

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListAIProvidersQuery 查询当前企业的 AI 供应商。
type ListAIProvidersQuery struct {
	db *bun.DB
}

// NewListAIProvidersQuery 创建 AI 供应商列表查询。
func NewListAIProvidersQuery(db *bun.DB) *ListAIProvidersQuery {
	return &ListAIProvidersQuery{db: db}
}

// Execute 返回当前企业的 AI 供应商列表。
func (q *ListAIProvidersQuery) Execute(ctx context.Context, identity *servermodels.Identity) ([]Summary, error) {
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return nil, err
	}
	providers := make([]servermodels.AIProvider, 0)
	if err := q.db.NewSelect().
		Model(&providers).
		Column("id", "brand", "name", "api_url").
		Where("aip.organization_id = ?", identity.Organization.ID).
		Order("aip.created_at ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list AI providers: %w", err)
	}
	modelRecords := make([]servermodels.AIProviderModel, 0)
	if err := q.db.NewSelect().
		Model(&modelRecords).
		Column("provider_id", "model_type").
		Where("aipm.organization_id = ?", identity.Organization.ID).
		Group("provider_id", "model_type").
		Order("provider_id ASC").
		Order("model_type ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list AI provider model types: %w", err)
	}
	modelTypesByProvider := make(map[string][]domain.AIModelType)
	for _, record := range modelRecords {
		modelTypesByProvider[record.ProviderID] = append(
			modelTypesByProvider[record.ProviderID],
			domain.AIModelType(record.Type),
		)
	}
	output := make([]Summary, 0, len(providers))
	for _, provider := range providers {
		output = append(output, Summary{
			ID: provider.ID, Brand: domain.AIProviderBrand(provider.Brand), Name: provider.Name, APIURL: provider.APIURL,
			ModelTypes: modelTypesByProvider[provider.ID],
		})
	}
	return output, nil
}
