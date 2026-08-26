//go:build server

package channel

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateMessageChannelStatusAction 修改消息渠道启用状态。
type UpdateMessageChannelStatusAction struct {
	db *bun.DB
}

// NewUpdateMessageChannelStatusAction 创建消息渠道状态操作。
func NewUpdateMessageChannelStatusAction(db *bun.DB) *UpdateMessageChannelStatusAction {
	return &UpdateMessageChannelStatusAction{db: db}
}

// Execute 修改当前企业中已支持消息渠道的启用状态。
func (a *UpdateMessageChannelStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, channelID string, enabled bool) (*MessageChannelRecord, error) {
	if !common.ValidUUID(channelID) {
		return nil, ErrNotFound
	}
	if err := identityaction.Validate(ctx, a.db, identity); err != nil {
		return nil, err
	}
	channel := &servermodels.Channel{}
	result, err := a.db.NewUpdate().
		Model(channel).
		Set("enabled = ?", enabled).
		Set("updated_at = now()").
		Where("c.id = ?", channelID).
		Where("c.organization_id = ?", identity.Organization.ID).
		Where("c.type IN (?)", bun.In(domain.MessageChannelTypes())).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update message channel status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read updated message channel count: %w", err)
	}
	if rows == 0 {
		return nil, ErrNotFound
	}
	return messageChannelRecord(channel), nil
}
