//go:build server

package channel

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ResetRoutingTarget 把引用指定对象的渠道路由重置为公共队列。
func ResetRoutingTarget(ctx context.Context, db bun.IDB, organizationID string, targetType domain.ChannelRoutingTargetType, targetID string) error {
	if _, err := db.NewUpdate().Model((*servermodels.Channel)(nil)).
		Set("initial_routing_target_type = ?", domain.ChannelRoutingTargetTypePublicQueue).
		Set("initial_routing_target_id = NULL").
		Set("updated_at = now()").
		Where("organization_id = ?", organizationID).
		Where("initial_routing_target_type = ?", targetType).
		Where("initial_routing_target_id = ?", targetID).
		Exec(ctx); err != nil {
		return err
	}
	_, err := db.NewUpdate().Model((*servermodels.Channel)(nil)).
		Set("fallback_routing_target_type = ?", domain.ChannelRoutingTargetTypePublicQueue).
		Set("fallback_routing_target_id = NULL").
		Set("updated_at = now()").
		Where("organization_id = ?", organizationID).
		Where("fallback_routing_target_type = ?", targetType).
		Where("fallback_routing_target_id = ?", targetID).
		Exec(ctx)
	return err
}
