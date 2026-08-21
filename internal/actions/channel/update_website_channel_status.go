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

// UpdateWebsiteChannelStatusAction 修改网站渠道启用状态。
type UpdateWebsiteChannelStatusAction struct {
	db *bun.DB
}

// NewUpdateWebsiteChannelStatusAction 创建网站渠道状态操作。
func NewUpdateWebsiteChannelStatusAction(db *bun.DB) *UpdateWebsiteChannelStatusAction {
	return &UpdateWebsiteChannelStatusAction{db: db}
}

// Execute 修改当前企业中网站渠道的启用状态。
func (a *UpdateWebsiteChannelStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, channelID string, enabled bool) (*servermodels.Channel, error) {
	if !common.ValidUUID(channelID) {
		return nil, ErrNotFound
	}
	channel := &servermodels.Channel{}
	result, err := a.db.NewUpdate().
		Model(channel).
		Set("enabled = ?", enabled).
		Set("updated_at = now()").
		Where("c.id = ?", channelID).
		Where("c.organization_id = ?", identity.Organization.ID).
		Where("c.type = ?", domain.ChannelTypeWebsite).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update website channel status: %w", err)
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
