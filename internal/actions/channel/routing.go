//go:build server

package channel

import (
	"context"
	"database/sql"
	"errors"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// validateRoutingTarget 校验会话流转目标属于当前企业且可用。
func validateRoutingTarget(ctx context.Context, db bun.IDB, organizationID string, channelType domain.ChannelType, field string, target RoutingTarget) error {
	if target.Type == domain.ChannelRoutingTargetTypePublicQueue {
		return nil
	}
	var available bool
	var err error
	switch target.Type {
	case domain.ChannelRoutingTargetTypeTeam:
		available, err = db.NewSelect().Model((*servermodels.Team)(nil)).
			Where("organization_id = ?", organizationID).
			Where("id = ?", target.ID).
			Exists(ctx)
	case domain.ChannelRoutingTargetTypeMember:
		identity, loadErr := identityaction.LockActiveCustomerServiceIdentity(ctx, db, organizationID, target.ID)
		if errors.Is(loadErr, sql.ErrNoRows) {
			break
		}
		if loadErr != nil {
			return loadErr
		}
		available = domain.OrganizationIdentityType(identity.Type) != domain.OrganizationIdentityTypeAgent || domain.ChannelSupportsAgentAssignee(channelType)
	}
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
