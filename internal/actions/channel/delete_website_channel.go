//go:build server

package channel

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DeleteWebsiteChannelAction 将网站渠道移入回收站。
type DeleteWebsiteChannelAction struct {
	db *bun.DB
}

// NewDeleteWebsiteChannelAction 创建网站渠道删除操作。
func NewDeleteWebsiteChannelAction(db *bun.DB) *DeleteWebsiteChannelAction {
	return &DeleteWebsiteChannelAction{db: db}
}

// Execute 软删除当前企业的网站渠道。
func (a *DeleteWebsiteChannelAction) Execute(ctx context.Context, principal *servermodels.Principal, channelID string) error {
	if !validUUID(channelID) {
		return ErrNotFound
	}
	result, err := a.db.NewUpdate().
		Table("channels").
		Set("deleted_at = now()").
		Set("updated_at = now()").
		Where("id = ?", channelID).
		Where("organization_id = ?", principal.Organization.ID).
		Where("type = ?", domain.ChannelTypeWebsite).
		Where("deleted_at IS NULL").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete website channel: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted website channel count: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
