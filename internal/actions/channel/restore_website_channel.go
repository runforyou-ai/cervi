//go:build server

package channel

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common/recordid"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// RestoreWebsiteChannelAction 从回收站恢复网站渠道。
type RestoreWebsiteChannelAction struct {
	db *bun.DB
}

// NewRestoreWebsiteChannelAction 创建网站渠道恢复操作。
func NewRestoreWebsiteChannelAction(db *bun.DB) *RestoreWebsiteChannelAction {
	return &RestoreWebsiteChannelAction{db: db}
}

// Execute 恢复当前企业中已软删除的网站渠道。
func (a *RestoreWebsiteChannelAction) Execute(ctx context.Context, identity *servermodels.Identity, channelID string) (*servermodels.Channel, error) {
	if !recordid.ValidUUID(channelID) {
		return nil, ErrNotFound
	}
	channel := &servermodels.Channel{}
	result, err := a.db.NewUpdate().
		Model(channel).
		Set("deleted_at = NULL").
		Set("updated_at = now()").
		Where("c.id = ?", channelID).
		Where("c.organization_id = ?", identity.Organization.ID).
		Where("c.type = ?", domain.ChannelTypeWebsite).
		Where("c.deleted_at IS NOT NULL").
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("restore website channel: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read restored website channel count: %w", err)
	}
	if rows == 0 {
		return nil, ErrNotFound
	}
	return channel, nil
}
