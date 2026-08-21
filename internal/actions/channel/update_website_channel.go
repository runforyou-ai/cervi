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
	query := a.db.NewUpdate().
		Model(channel).
		Set("name = ?", input.Name).
		Set("description = NULL").
		Set("default_locale = ?", input.DefaultLocale).
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
		return nil, fmt.Errorf("update website channel: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read updated website channel count: %w", err)
	}
	if rows == 0 {
		return nil, ErrNotFound
	}
	return channel, nil
}
