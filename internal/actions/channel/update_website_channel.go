//go:build server

package channel

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateWebsiteChannelAction 修改网站渠道基础信息。
type UpdateWebsiteChannelAction struct {
	db *bun.DB
}

// NewUpdateWebsiteChannelAction 创建网站渠道修改操作。
func NewUpdateWebsiteChannelAction(db *bun.DB) *UpdateWebsiteChannelAction {
	return &UpdateWebsiteChannelAction{db: db}
}

// Execute 校验并更新当前企业的网站渠道。
func (a *UpdateWebsiteChannelAction) Execute(ctx context.Context, identity *servermodels.Identity, channelID string, input WebsiteChannelInput) (*servermodels.Channel, error) {
	if !common.ValidUUID(channelID) {
		return nil, ErrNotFound
	}
	input, fields := normalizeWebsiteChannelInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}

	channel := &servermodels.Channel{}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateRoutingTarget(ctx, tx, identity.Organization.ID, "newConversationTarget", input.NewConversationTarget); err != nil {
			return err
		}
		if err := validateRoutingTarget(ctx, tx, identity.Organization.ID, "fallbackTarget", input.FallbackTarget); err != nil {
			return err
		}
		query := tx.NewUpdate().
			Model(channel).
			Set("name = ?", input.Name).
			Set("description = NULL").
			Set("default_locale = ?", input.DefaultLocale).
			Set("initial_routing_target_type = ?", input.NewConversationTarget.Type).
			Set("initial_routing_target_id = ?", routingTargetID(input.NewConversationTarget)).
			Set("fallback_routing_target_type = ?", input.FallbackTarget.Type).
			Set("fallback_routing_target_id = ?", routingTargetID(input.FallbackTarget)).
			Set("updated_at = now()")
		if input.Description != "" {
			query = query.Set("description = ?", input.Description)
		}
		result, err := query.
			Where("c.id = ?", channelID).
			Where("c.organization_id = ?", identity.Organization.ID).
			Where("c.type = ?", domain.ChannelTypeWebsite).
			Returning("*").
			Exec(ctx)
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
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update website channel: %w", err)
	}
	return channel, nil
}
