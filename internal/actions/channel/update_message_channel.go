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

// UpdateMessageChannelAction 修改消息渠道基础信息。
type UpdateMessageChannelAction struct {
	db *bun.DB
}

// NewUpdateMessageChannelAction 创建消息渠道修改操作。
func NewUpdateMessageChannelAction(db *bun.DB) *UpdateMessageChannelAction {
	return &UpdateMessageChannelAction{db: db}
}

// Execute 校验并更新当前企业中受支持的消息渠道。
func (a *UpdateMessageChannelAction) Execute(ctx context.Context, identity *servermodels.Identity, channelID string, input MessageChannelInput) (*servermodels.Channel, error) {
	if !common.ValidUUID(channelID) {
		return nil, ErrNotFound
	}
	input, fields := normalizeMessageChannelInput(input)
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
		var description *string
		if input.Description != "" {
			description = &input.Description
		}
		result, err := tx.NewUpdate().
			Model(channel).
			Set("name = ?", input.Name).
			Set("description = ?", description).
			Set("default_locale = ?", input.DefaultLocale).
			Set("initial_routing_target_type = ?", input.NewConversationTarget.Type).
			Set("initial_routing_target_id = ?", routingTargetID(input.NewConversationTarget)).
			Set("fallback_routing_target_type = ?", input.FallbackTarget.Type).
			Set("fallback_routing_target_id = ?", routingTargetID(input.FallbackTarget)).
			Set("updated_at = now()").
			Where("c.id = ?", channelID).
			Where("c.organization_id = ?", identity.Organization.ID).
			Where("c.type IN (?)", bun.In(domain.MessageChannelTypes())).
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
		return nil, fmt.Errorf("update message channel: %w", err)
	}
	return channel, nil
}
