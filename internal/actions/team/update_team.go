//go:build server

package team

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateTeamAction 修改企业团队。
type UpdateTeamAction struct{ db *bun.DB }

// NewUpdateTeamAction 创建团队修改操作。
func NewUpdateTeamAction(db *bun.DB) *UpdateTeamAction { return &UpdateTeamAction{db: db} }

// Execute 校验并修改当前企业的团队。
func (a *UpdateTeamAction) Execute(ctx context.Context, identity *servermodels.Identity, teamID string, input Input) (*TeamRecord, error) {
	input, fields := normalizeInput(input)
	if len(fields) > 0 {
		return nil, &common.FieldError{Fields: fields}
	}
	if !common.ValidUUID(teamID) {
		return nil, ErrNotFound
	}
	var record *TeamRecord
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		result, err := tx.NewUpdate().Model((*servermodels.Team)(nil)).
			Set("name = ?", input.Name).
			Set("description = ?", input.Description).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", teamID).
			Exec(ctx)
		if isUniqueViolation(err) {
			return &common.FieldError{Fields: map[string]common.FieldCode{"name": ValidationNameDuplicate}}
		}
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrNotFound
		}
		record, err = loadTeam(ctx, tx, identity.Organization.ID, teamID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update team: %w", err)
	}
	return record, nil
}
