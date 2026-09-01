//go:build server

package team

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// RemoveMembersAction 批量移出团队成员。
type RemoveMembersAction struct{ db *bun.DB }

// NewRemoveMembersAction 创建团队成员批量移出操作。
func NewRemoveMembersAction(db *bun.DB) *RemoveMembersAction {
	return &RemoveMembersAction{db: db}
}

// Execute 规范化并校验成员后批量解除团队关系。
func (a *RemoveMembersAction) Execute(ctx context.Context, identity *servermodels.Identity, teamID string, members []MemberIdentity) (*TeamRecord, error) {
	var team *TeamRecord
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := loadTeam(ctx, tx, identity.Organization.ID, teamID); err != nil {
			return err
		}
		unique := make(map[string]MemberIdentity, len(members))
		for _, member := range members {
			identityID, identityIDValid := common.NormalizeUUID(member.IdentityID)
			if (member.IdentityType != domain.OrganizationIdentityTypeUser && member.IdentityType != domain.OrganizationIdentityTypeAgent) || !identityIDValid {
				return ErrMemberInvalid
			}
			member.IdentityID = identityID
			unique[member.IdentityID] = member
		}
		if len(unique) == 0 {
			return ErrMemberInvalid
		}
		for _, member := range unique {
			exists, err := tx.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).
				Where("organization_id = ?", identity.Organization.ID).
				Where("id = ?", member.IdentityID).
				Where("type = ?", member.IdentityType).
				Exists(ctx)
			if err != nil {
				return err
			}
			if !exists {
				return ErrMemberInvalid
			}
			result, err := tx.NewDelete().Model((*servermodels.TeamMember)(nil)).
				Where("organization_id = ?", identity.Organization.ID).
				Where("team_id = ?", teamID).
				Where("identity_id = ?", member.IdentityID).
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
		}
		var err error
		team, err = loadTeam(ctx, tx, identity.Organization.ID, teamID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("remove team members: %w", err)
	}
	return team, nil
}
