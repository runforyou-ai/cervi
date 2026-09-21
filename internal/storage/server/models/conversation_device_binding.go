//go:build server

package models

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/uptrace/bun"
)

// ConversationDeviceBinding 表示 PostgreSQL 中会话绑定的执行设备与工作区。
type ConversationDeviceBinding struct {
	bun.BaseModel `bun:"table:conversation_device_bindings,alias:cdb"`

	ID                    string                       `bun:"id,pk"`
	OrganizationID        string                       `bun:"organization_id"`
	ConversationID        string                       `bun:"conversation_id"`
	DeviceID              string                       `bun:"device_id"`
	WorkspaceID           string                       `bun:"workspace_id"`
	PeerTriggerCapability domain.PeerTriggerCapability `bun:"peer_trigger_capability"`
	BoundByUserID         string                       `bun:"bound_by_user_id"`
	CreatedAt             time.Time                    `bun:"created_at"`
	UpdatedAt             time.Time                    `bun:"updated_at"`
}
