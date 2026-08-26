//go:build server

package team

import (
	"context"
	"fmt"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DeleteTeamAction 删除企业团队。
type DeleteTeamAction struct{ db *bun.DB }

// NewDeleteTeamAction 创建团队删除操作。
func NewDeleteTeamAction(db *bun.DB) *DeleteTeamAction { return &DeleteTeamAction{db: db} }

// Execute 删除团队及其成员关系，并把渠道关联重置到公共队列。
func (a *DeleteTeamAction) Execute(ctx context.Context, identity *servermodels.Identity, teamID string) error {
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := loadTeam(ctx, tx, identity.Organization.ID, teamID); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*servermodels.TeamMember)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("team_id = ?", teamID).
			Exec(ctx); err != nil {
			return err
		}
		if err := channelaction.ResetRoutingTarget(ctx, tx, identity.Organization.ID, domain.ChannelRoutingTargetTypeTeam, teamID); err != nil {
			return err
		}
		_, err := tx.NewDelete().Model((*servermodels.Team)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", teamID).
			Exec(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	return nil
}
