//go:build server

package channel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateWebsiteChannelChatInterfaceAction 修改网站渠道聊天界面。
type UpdateWebsiteChannelChatInterfaceAction struct {
	db *bun.DB
}

// NewUpdateWebsiteChannelChatInterfaceAction 创建聊天界面修改操作。
func NewUpdateWebsiteChannelChatInterfaceAction(db *bun.DB) *UpdateWebsiteChannelChatInterfaceAction {
	return &UpdateWebsiteChannelChatInterfaceAction{db: db}
}

// Execute 校验渠道归属并保存聊天界面设置。
func (a *UpdateWebsiteChannelChatInterfaceAction) Execute(ctx context.Context, identity *servermodels.Identity, channelID string, input WebsiteChannelChatInterfaceInput) (*WebsiteChannelSettingRecord, error) {
	if !common.ValidUUID(channelID) {
		return nil, ErrNotFound
	}
	input, fields := normalizeWebsiteChannelChatInterfaceInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}

	setting := &servermodels.WebsiteChannelSetting{}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		channel := &servermodels.Channel{}
		if err := tx.NewSelect().
			Model(channel).
			Column("id").
			Where("c.id = ?", channelID).
			Where("c.organization_id = ?", identity.Organization.ID).
			Where("c.type = ?", domain.ChannelTypeWebsite).
			For("UPDATE").
			Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		var subtitle *string
		if input.Subtitle != "" {
			subtitle = &input.Subtitle
		}
		var greetingMessage *string
		if input.GreetingMessage != "" {
			greetingMessage = &input.GreetingMessage
		}
		return tx.NewUpdate().
			Model(setting).
			Set("chat_title = ?", input.Title).
			Set("chat_subtitle = ?", subtitle).
			Set("greeting_message = ?", greetingMessage).
			Set("theme_color = ?", input.ThemeColor).
			Set("updated_at = now()").
			Where("wcs.channel_id = ?", channelID).
			Where("wcs.organization_id = ?", identity.Organization.ID).
			Returning("*").
			Scan(ctx)
	})
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update website channel chat interface: %w", err)
	}
	record := websiteChannelSettingRecord(setting)
	return &record, nil
}
