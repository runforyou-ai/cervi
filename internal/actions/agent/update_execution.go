//go:build server

package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateExecutionAction 修改 AI 员工当前生效的执行配置。
type UpdateExecutionAction struct{ db *bun.DB }

// NewUpdateExecutionAction 创建 AI 员工执行配置修改操作。
func NewUpdateExecutionAction(db *bun.DB) *UpdateExecutionAction {
	return &UpdateExecutionAction{db: db}
}

// Execute 创建新执行配置版本并切换 AI 员工的当前版本。
func (a *UpdateExecutionAction) Execute(ctx context.Context, identity *servermodels.Identity, agentID string, input ExecutionInput) (*Agent, error) {
	if !common.ValidUUID(agentID) {
		return nil, ErrNotFound
	}
	input, err := normalizeExecutionInput(input)
	if err != nil {
		return nil, err
	}
	var output *Agent
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		stored := &servermodels.Agent{}
		err = tx.NewSelect().Model(stored).
			Column("id").
			Where("a.organization_id = ?", identity.Organization.ID).
			Where("a.id = ?", agentID).
			For("UPDATE").
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		model, err := loadManagedExecutionModel(ctx, tx, identity.Organization.ID, *input.Managed)
		if err != nil {
			return err
		}
		revisionID, err := uuid.NewV7()
		if err != nil {
			return err
		}
		execution, err := insertExecutionRevision(ctx, tx, identity, agentID, revisionID.String(), input, model)
		if err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*servermodels.Agent)(nil)).
			Set("active_revision_id = ?", execution.RevisionID).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", agentID).
			Exec(ctx); err != nil {
			return err
		}
		output, err = loadAgent(ctx, tx, identity.Organization.ID, agentID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update agent execution: %w", err)
	}
	return output, nil
}
