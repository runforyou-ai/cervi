//go:build server

package channel

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateMessageChannelAction 创建消息渠道。
type CreateMessageChannelAction struct {
	db *bun.DB
}

// NewCreateMessageChannelAction 创建消息渠道操作。
func NewCreateMessageChannelAction(db *bun.DB) *CreateMessageChannelAction {
	return &CreateMessageChannelAction{db: db}
}

// Execute 创建消息渠道，并初始化当前渠道类型需要的设置。
func (a *CreateMessageChannelAction) Execute(ctx context.Context, identity *servermodels.Identity, input CreateMessageChannelInput) (*MessageChannelRecord, error) {
	input, fields := normalizeCreateMessageChannelInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	channel := &servermodels.Channel{
		OrganizationID:            identity.Organization.ID,
		CreatedByUserID:           identity.User.ID,
		Type:                      string(input.Type),
		Name:                      input.Name,
		DefaultLocale:             string(input.DefaultLocale),
		InitialRoutingTargetType:  string(input.NewConversationTarget.Type),
		InitialRoutingTargetID:    routingTargetID(input.NewConversationTarget),
		FallbackRoutingTargetType: string(input.FallbackTarget.Type),
		FallbackRoutingTargetID:   routingTargetID(input.FallbackTarget),
	}
	if input.Description != "" {
		channel.Description = &input.Description
	}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		if err := validateRoutingTarget(ctx, tx, identity.Organization.ID, "newConversationTarget", input.NewConversationTarget); err != nil {
			return err
		}
		if err := validateRoutingTarget(ctx, tx, identity.Organization.ID, "fallbackTarget", input.FallbackTarget); err != nil {
			return err
		}

		_, err := tx.NewInsert().
			Model(channel).
			Column("organization_id", "created_by_user_id", "type", "name", "description", "default_locale", "initial_routing_target_type", "initial_routing_target_id", "fallback_routing_target_type", "fallback_routing_target_id").
			Returning("*").
			Exec(ctx)
		if err != nil {
			return err
		}
		if input.Type != domain.ChannelTypeWebsite {
			return nil
		}

		setting := &servermodels.WebsiteChannelSetting{
			ChannelID:      channel.ID,
			OrganizationID: identity.Organization.ID,
			ChatTitle:      channel.Name,
			ThemeColor:     DefaultWebsiteChannelThemeColor,
		}
		_, err = tx.NewInsert().
			Model(setting).
			Column("channel_id", "organization_id", "chat_title", "theme_color").
			Exec(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create message channel: %w", err)
	}
	return messageChannelRecord(channel), nil
}
