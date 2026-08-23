//go:build server

package channel

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// validateRoutingTarget 校验会话流转目标属于当前企业且可用。
func validateRoutingTarget(ctx context.Context, db bun.IDB, organizationID, field string, target RoutingTarget) error {
	if target.Type == domain.ChannelRoutingTargetTypePublicQueue {
		return nil
	}
	var query *bun.SelectQuery
	switch target.Type {
	case domain.ChannelRoutingTargetTypeTeam:
		query = db.NewSelect().Model((*servermodels.Team)(nil)).
			Where("organization_id = ?", organizationID).
			Where("id = ?", target.ID)
	case domain.ChannelRoutingTargetTypeMember:
		query = db.NewSelect().TableExpr("organization_identities AS oi").
			Join("LEFT JOIN users AS u ON u.identity_id = oi.id AND u.organization_id = oi.organization_id").
			Join("LEFT JOIN agents AS a ON a.identity_id = oi.id AND a.organization_id = oi.organization_id").
			Where("oi.organization_id = ?", organizationID).
			Where("oi.id = ?", target.ID).
			Where("((oi.type = ? AND u.status = ?) OR (oi.type = ? AND a.status = ?))", domain.OrganizationIdentityTypeUser, domain.UserStatusActive, domain.OrganizationIdentityTypeAgent, domain.UserStatusActive)
	}
	available, err := query.Exists(ctx)
	if err != nil {
		return err
	}
	if !available {
		return &ValidationError{Fields: map[string]ValidationCode{field: ValidationRoutingTargetInvalid}}
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
