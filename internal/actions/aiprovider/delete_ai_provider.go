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

// DeleteAIProviderAction 删除模型服务供应商。
type DeleteAIProviderAction struct {
	db *bun.DB
}

// NewDeleteAIProviderAction 创建模型服务供应商删除操作。
func NewDeleteAIProviderAction(db *bun.DB) *DeleteAIProviderAction {
	return &DeleteAIProviderAction{db: db}
}

// Execute 删除模型服务供应商及其模型目录。
func (a *DeleteAIProviderAction) Execute(ctx context.Context, identity *servermodels.Identity, providerID string) error {
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		provider, err := loadProvider(ctx, tx, identity.Organization.ID, providerID, true)
		if err != nil {
			return err
		}
		// 判断供应商是否被 AI 员工使用。
		inUse, err := tx.NewSelect().TableExpr("agents AS a").
			Join("JOIN agent_revisions AS ar ON ar.id = a.active_revision_id AND ar.organization_id = a.organization_id AND ar.agent_id = a.id").
			Where("a.organization_id = ?", identity.Organization.ID).
			Where("ar.execution_mode = ?", domain.AgentExecutionModeManaged).
			Where("ar.configuration #>> '{model,providerId}' = ?", provider.ID).
			Exists(ctx)
		if err != nil {
			return err
		}
		if inUse {
			return ErrInUse
		}
		if _, err := tx.NewDelete().
			Model((*servermodels.AIProviderModel)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("provider_id = ?", provider.ID).
			Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewDelete().
			Model(provider).
			Where("organization_id = ?", identity.Organization.ID).
			WherePK().
			Exec(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("delete AI provider: %w", err)
	}
	return nil
}
