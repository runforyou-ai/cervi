//go:build server

package channel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityerr "github.com/runforyou-ai/cervi/internal/common/identity"
	"github.com/runforyou-ai/cervi/internal/common/recordid"
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
		!recordid.ValidUUID(identity.Organization.ID) ||
		!recordid.ValidUUID(identity.User.ID) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return nil, identityerr.ErrInvalid
	}

	channel := &servermodels.Channel{
		OrganizationID:  identity.Organization.ID,
		CreatedByUserID: identity.User.ID,
		Type:            string(domain.ChannelTypeWebsite),
		Name:            input.Name,
		DefaultLocale:   string(input.DefaultLocale),
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
				return identityerr.ErrInvalid
			}
			return err
		}

		user := &servermodels.User{}
		if err := tx.NewSelect().
			Model(user).
			Column("id").
			Where("u.id = ?", identity.User.ID).
			Where("u.organization_id = ?", identity.Organization.ID).
			For("KEY SHARE").
			Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return identityerr.ErrInvalid
			}
			return err
		}

		_, err := tx.NewInsert().
			Model(channel).
			Column("organization_id", "created_by_user_id", "type", "name", "description", "default_locale").
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
	if errors.Is(err, identityerr.ErrInvalid) {
		return nil, identityerr.ErrInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("create website channel: %w", err)
	}
	return channel, nil
}
