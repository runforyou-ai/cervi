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

// AddMembersAction 将企业身份批量加入团队。
type AddMembersAction struct{ db *bun.DB }

// NewAddMembersAction 创建团队成员添加操作。
func NewAddMembersAction(db *bun.DB) *AddMembersAction { return &AddMembersAction{db: db} }

// Execute 规范化并校验成员后批量建立团队关系。
func (a *AddMembersAction) Execute(ctx context.Context, identity *servermodels.Identity, teamID string, members []MemberIdentity) (*TeamRecord, error) {
	var team *TeamRecord
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := loadTeam(ctx, tx, identity.Organization.ID, teamID); err != nil {
			return err
		}
		uniqueIDs := make(map[string]domain.OrganizationIdentityType, len(members))
		for _, member := range members {
			identityID, identityIDValid := common.NormalizeUUID(member.IdentityID)
			if (member.IdentityType != domain.OrganizationIdentityTypeUser && member.IdentityType != domain.OrganizationIdentityTypeAgent) || !identityIDValid {
				return ErrMemberInvalid
			}
			uniqueIDs[identityID] = member.IdentityType
		}
		if len(uniqueIDs) == 0 {
			return ErrMemberInvalid
		}
		ids := make([]string, 0, len(uniqueIDs))
		for id := range uniqueIDs {
			ids = append(ids, id)
		}
		var storedIdentities []servermodels.OrganizationIdentity
		err := tx.NewSelect().Model(&storedIdentities).
			ColumnExpr("oi.id, oi.type").
			Join("LEFT JOIN users AS u ON u.identity_id = oi.id AND u.organization_id = oi.organization_id").
			Join("LEFT JOIN agents AS a ON a.identity_id = oi.id AND a.organization_id = oi.organization_id").
			Where("oi.organization_id = ?", identity.Organization.ID).
			Where("((oi.type = ? AND u.status = ?) OR (oi.type = ? AND a.status = ?))", domain.OrganizationIdentityTypeUser, domain.UserStatusActive, domain.OrganizationIdentityTypeAgent, domain.UserStatusActive).
			Where("oi.id IN (?)", bun.In(ids)).
			Scan(ctx)
		if err != nil {
			return err
		}
		if len(storedIdentities) != len(ids) {
			return ErrMemberInvalid
		}
		for _, storedIdentity := range storedIdentities {
			if uniqueIDs[storedIdentity.ID] != domain.OrganizationIdentityType(storedIdentity.Type) {
				return ErrMemberInvalid
			}
		}
		relations := make([]servermodels.TeamMember, 0, len(ids))
		for _, id := range ids {
			relations = append(relations, servermodels.TeamMember{
				OrganizationID:  identity.Organization.ID,
				TeamID:          teamID,
				IdentityID:      id,
				CreatedByUserID: identity.User.ID,
			})
		}
		if _, err := tx.NewInsert().Model(&relations).
			Column("organization_id", "team_id", "identity_id", "created_by_user_id").
			On("CONFLICT (organization_id, team_id, identity_id) DO NOTHING").
			Exec(ctx); err != nil {
			return err
		}
		team, err = loadTeam(ctx, tx, identity.Organization.ID, teamID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("add team members: %w", err)
	}
	return team, nil
}
