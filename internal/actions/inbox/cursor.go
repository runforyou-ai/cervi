//go:build server

package inbox

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

const inboxSortVersion = 1

// compareInboxPoints 按活动降序和编号降序比较位置，空活动时间排在最后。
func compareInboxPoints(left, right inboxCursorPoint) int {
	if left.LastActivityAt == nil && right.LastActivityAt != nil {
		return 1
	}
	if left.LastActivityAt != nil && right.LastActivityAt == nil {
		return -1
	}
	if left.LastActivityAt != nil && right.LastActivityAt != nil {
		if order := left.LastActivityAt.Compare(*right.LastActivityAt); order != 0 {
			return -order
		}
	}
	return -strings.Compare(left.ID, right.ID)
}

// inboxCursorPoint 保存数据库原始活动时间，空时间使用独立的编号边界。
type inboxCursorPoint struct {
	ID             string     `json:"id" bun:"id"`
	LastActivityAt *time.Time `json:"lastActivityAt" bun:"last_activity_at"`
}

// inboxCursor 将排序边界绑定到当前企业、用户和规范化筛选。
type inboxCursor struct {
	inboxCursorPoint
	Version            int                         `json:"version"`
	OrganizationID     string                      `json:"organizationId"`
	UserID             string                      `json:"userId"`
	Scope              domain.InboxScope           `json:"scope"`
	CustomerView       domain.CustomerInboxView    `json:"customerView"`
	AssigneeIdentityID string                      `json:"assigneeIdentityId"`
	ChannelID          string                      `json:"channelId"`
	ServiceStatus      domain.ServiceSessionStatus `json:"serviceStatus"`
	Kinds              []domain.ConversationType   `json:"kinds"`
}

// encodeInboxCursor 编码原始活动边界、身份范围和排序版本。
func encodeInboxCursor(identity *servermodels.Identity, input LoadInput, point inboxCursorPoint) (string, error) {
	data, err := json.Marshal(inboxCursor{
		inboxCursorPoint: point, Version: inboxSortVersion,
		OrganizationID: identity.Organization.ID, UserID: identity.User.ID,
		Scope: input.Scope, CustomerView: input.CustomerView, AssigneeIdentityID: input.AssigneeIdentityID,
		ChannelID: input.ChannelID, ServiceStatus: input.ServiceStatus, Kinds: input.Kinds,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// decodeInboxCursor 拒绝跨查询、跨用户、跨企业及未知排序版本的游标。
func decodeInboxCursor(value string, identity *servermodels.Identity, input LoadInput) (*inboxCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrCursorInvalid
	}
	var cursor inboxCursor
	if json.Unmarshal(data, &cursor) != nil || cursor.Version != inboxSortVersion ||
		cursor.OrganizationID != identity.Organization.ID || cursor.UserID != identity.User.ID ||
		cursor.Scope != input.Scope || cursor.CustomerView != input.CustomerView || cursor.AssigneeIdentityID != input.AssigneeIdentityID ||
		cursor.ChannelID != input.ChannelID || cursor.ServiceStatus != input.ServiceStatus || !slices.Equal(cursor.Kinds, input.Kinds) ||
		!common.ValidUUID(cursor.ID) {
		return nil, ErrCursorInvalid
	}
	// 数据库 UUID 返回小写，统一比较与锚点匹配所用的编号。
	cursor.ID = strings.ToLower(cursor.ID)
	return &cursor, nil
}
