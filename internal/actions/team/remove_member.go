//go:build server

package team

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// RemoveMemberAction 移出团队成员。
type RemoveMemberAction struct{ db *bun.DB }

// NewRemoveMemberAction 创建团队成员移出操作。
func NewRemoveMemberAction(db *bun.DB) *RemoveMemberAction { return &RemoveMemberAction{db: db} }

// Execute 解除成员与团队的关系。
func (a *RemoveMemberAction) Execute(ctx context.Context, identity *servermodels.Identity, teamID string, identityType domain.MemberIdentityType, identityID string) error {
	if (identityType != domain.MemberIdentityTypeUser && identityType != domain.MemberIdentityTypeAgent) || !common.ValidUUID(identityID) {
		return ErrMemberNotFound
	}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := loadTeam(ctx, tx, identity.Organization.ID, teamID); err != nil {
			return err
		}
		result, err := tx.NewDelete().Model((*servermodels.TeamMember)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("team_id = ?", teamID).
			Where("identity_type = ?", identityType).
			Where("identity_id = ?", identityID).
			Exec(ctx)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrMemberNotFound
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}
	return nil
}
