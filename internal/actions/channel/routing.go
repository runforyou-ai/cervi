//go:build server

package channel

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// validateRoutingTarget 校验会话流转目标属于当前企业且可用。
func validateRoutingTarget(ctx context.Context, db bun.IDB, organizationID, field string, target RoutingTarget) error {
	if target.Type == domain.ChannelRoutingTargetTypePublicQueue {
		return nil
	}
	var err error
	switch target.Type {
	case domain.ChannelRoutingTargetTypeTeam:
		record := &servermodels.Team{}
		err = db.NewSelect().Model(record).Column("id").
			Where("organization_id = ?", organizationID).
			Where("id = ?", target.ID).
			For("KEY SHARE").Scan(ctx)
	case domain.ChannelRoutingTargetTypeMember:
		record := &servermodels.OrganizationIdentity{}
		err = db.NewSelect().Model(record).Column("id", "type").
			Where("organization_id = ?", organizationID).
			Where("id = ?", target.ID).
			For("KEY SHARE").Scan(ctx)
		if err == nil {
			switch domain.OrganizationIdentityType(record.Type) {
			case domain.OrganizationIdentityTypeUser:
				user := &servermodels.User{}
				err = db.NewSelect().Model(user).Column("id").
					Where("organization_id = ?", organizationID).
					Where("identity_id = ?", target.ID).
					Where("status = ?", domain.UserStatusActive).
					For("KEY SHARE").Scan(ctx)
			case domain.OrganizationIdentityTypeAgent:
				agent := &servermodels.Agent{}
				err = db.NewSelect().Model(agent).Column("id").
					Where("organization_id = ?", organizationID).
					Where("identity_id = ?", target.ID).
					Where("status = ?", domain.UserStatusActive).
					For("KEY SHARE").Scan(ctx)
			default:
				err = sql.ErrNoRows
			}
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return &ValidationError{Fields: map[string]ValidationCode{field: ValidationRoutingTargetInvalid}}
	}
	if err != nil {
		return err
	}
	return nil
}

// routingTargetID 返回可写入渠道记录的目标编号。
func routingTargetID(target RoutingTarget) *string {
	if target.Type == domain.ChannelRoutingTargetTypePublicQueue {
		return nil
	}
	return &target.ID
}
