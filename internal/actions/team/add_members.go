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

// AddMembersAction 将企业成员批量加入团队。
type AddMembersAction struct{ db *bun.DB }

// NewAddMembersAction 创建团队成员添加操作。
func NewAddMembersAction(db *bun.DB) *AddMembersAction { return &AddMembersAction{db: db} }

// Execute 校验成员并批量建立团队关系。
func (a *AddMembersAction) Execute(ctx context.Context, identity *servermodels.Identity, teamID string, members []MemberIdentity) (*TeamRecord, error) {
	var team *TeamRecord
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := loadTeam(ctx, tx, identity.Organization.ID, teamID); err != nil {
			return err
		}
		uniqueIDs := make(map[string]domain.MemberIdentityType, len(members))
		for _, member := range members {
			if (member.Type != domain.MemberIdentityTypeUser && member.Type != domain.MemberIdentityTypeAgent) || !common.ValidUUID(member.ID) {
				return ErrMemberInvalid
			}
			uniqueIDs[member.ID] = member.Type
		}
		if len(uniqueIDs) == 0 {
			return ErrMemberInvalid
		}
		ids := make([]string, 0, len(uniqueIDs))
		for id := range uniqueIDs {
			ids = append(ids, id)
		}
		var storedMembers []servermodels.OrganizationMember
		err := tx.NewSelect().Model(&storedMembers).
			Column("id", "type").
			Where("organization_id = ?", identity.Organization.ID).
			Where("status = 'active'").
			Where("id IN (?)", bun.In(ids)).
			Scan(ctx)
		if err != nil {
			return err
		}
		if len(storedMembers) != len(ids) {
			return ErrMemberInvalid
		}
		for _, storedMember := range storedMembers {
			if uniqueIDs[storedMember.ID] != domain.MemberIdentityType(storedMember.Type) {
				return ErrMemberInvalid
			}
		}
		relations := make([]servermodels.TeamMember, 0, len(ids))
		for _, id := range ids {
			relations = append(relations, servermodels.TeamMember{
				OrganizationID:  identity.Organization.ID,
				TeamID:          teamID,
				MemberID:        id,
				CreatedByUserID: identity.User.ID,
			})
		}
		if _, err := tx.NewInsert().Model(&relations).
			Column("organization_id", "team_id", "member_id", "created_by_user_id").
			On("CONFLICT (organization_id, team_id, member_id) DO NOTHING").
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
