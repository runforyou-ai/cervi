//go:build server

package channel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateWebsiteChannelAction 创建网站消息渠道。
type CreateWebsiteChannelAction struct {
	db *bun.DB
}

// NewCreateWebsiteChannelAction 创建网站渠道操作。
func NewCreateWebsiteChannelAction(db *bun.DB) *CreateWebsiteChannelAction {
	return &CreateWebsiteChannelAction{db: db}
}

// Execute 创建网站渠道及默认聊天界面。
func (a *CreateWebsiteChannelAction) Execute(ctx context.Context, identity *servermodels.Identity, input WebsiteChannelInput) (*servermodels.Channel, error) {
	input, fields := normalizeWebsiteChannelInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if identity == nil ||
		!common.ValidUUID(identity.Organization.ID) ||
		!common.ValidUUID(identity.User.ID) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return nil, common.ErrIdentityInvalid
	}

	channel := &servermodels.Channel{
		OrganizationID:            identity.Organization.ID,
		CreatedByUserID:           identity.User.ID,
		Type:                      string(input.Type),
		Name:                      input.Name,
		DefaultLocale:             string(input.DefaultLocale),
		NewConversationTargetType: string(input.NewConversationTarget.Type),
		NewConversationTargetID:   routingTargetID(input.NewConversationTarget),
		FallbackTargetType:        string(input.FallbackTarget.Type),
		FallbackTargetID:          routingTargetID(input.FallbackTarget),
	}
	if input.Description != "" {
		channel.Description = &input.Description
	}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		organization := &servermodels.Organization{}
		if err := tx.NewSelect().
			Model(organization).
			Column("id").
			Where("o.id = ?", identity.Organization.ID).
			For("KEY SHARE").
			Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return common.ErrIdentityInvalid
			}
			return err
		}

		user := &servermodels.OrganizationMember{}
		if err := tx.NewSelect().
			Model(user).
			Column("id").
			Where("om.id = ?", identity.User.ID).
			Where("om.organization_id = ?", identity.Organization.ID).
			Where("om.type = ?", domain.MemberIdentityTypeUser).
			Where("om.status = ?", domain.UserStatusActive).
			For("KEY SHARE").
			Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return common.ErrIdentityInvalid
			}
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
			Column("organization_id", "created_by_user_id", "type", "name", "description", "default_locale", "new_conversation_target_type", "new_conversation_target_id", "fallback_target_type", "fallback_target_id").
			Returning("*").
			Exec(ctx)
		if err != nil {
			return err
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
	if errors.Is(err, common.ErrIdentityInvalid) {
		return nil, common.ErrIdentityInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("create website channel: %w", err)
	}
	return channel, nil
}
