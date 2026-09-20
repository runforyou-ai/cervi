//go:build server

package inbox

import (
	"cmp"
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

// compareInboxPoints 按当前分区的显示顺序比较位置，结果为负表示排在更前面。
func compareInboxPoints(partition domain.InboxPartition, left, right inboxCursorPoint) int {
	if partition == domain.InboxPartitionPinned {
		if order := comparePinRanks(left.PinRank, right.PinRank); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	}
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

// comparePinRanks 按顺序值升序比较，空顺序值排在最后。
func comparePinRanks(left, right *int64) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return 1
	}
	if right == nil {
		return -1
	}
	return cmp.Compare(*left, *right)
}

// inboxCursorPoint 保存数据库原始活动时间与个人置顶顺序值，空活动时间使用独立的编号边界。
type inboxCursorPoint struct {
	ID             string     `json:"id" bun:"id"`
	LastActivityAt *time.Time `json:"lastActivityAt" bun:"last_activity_at"`
	PinRank        *int64     `json:"pinRank" bun:"pin_rank"`
}

// inboxCursor 将排序边界绑定到当前企业、用户、规范化筛选、搜索词和置顶分区。
type inboxCursor struct {
	inboxCursorPoint
	Version            int                         `json:"version"`
	OrganizationID     string                      `json:"organizationId"`
	UserID             string                      `json:"userId"`
	Partition          domain.InboxPartition       `json:"partition"`
	PinOrderVersion    int64                       `json:"pinOrderVersion"`
	Scope              domain.InboxScope           `json:"scope"`
	CustomerView       domain.CustomerInboxView    `json:"customerView"`
	QueueFilter        domain.CustomerQueueFilter  `json:"queueFilter"`
	QueueTeamID        string                      `json:"queueTeamId"`
	AssigneeIdentityID string                      `json:"assigneeIdentityId"`
	ChannelID          string                      `json:"channelId"`
	ServiceStatus      domain.ServiceSessionStatus `json:"serviceStatus"`
	Kinds              []domain.ConversationType   `json:"kinds"`
	Search             string                      `json:"search"`
	SearchRange        SearchRange                 `json:"searchRange"`
}

// encodeInboxCursor 编码原始排序边界、身份范围、搜索词、分区与排序版本；置顶区另外绑定个人顺序版本。
func encodeInboxCursor(identity *servermodels.Identity, input LoadInput, pinOrderVersion int64, point inboxCursorPoint) (string, error) {
	if input.Partition != domain.InboxPartitionPinned {
		pinOrderVersion = 0
	}
	data, err := json.Marshal(inboxCursor{
		inboxCursorPoint: point, Version: inboxSortVersion,
		OrganizationID: identity.Organization.ID, UserID: identity.User.ID,
		Partition: input.Partition, PinOrderVersion: pinOrderVersion,
		Scope: input.Scope, CustomerView: input.CustomerView, AssigneeIdentityID: input.AssigneeIdentityID,
		QueueFilter: input.QueueFilter, QueueTeamID: input.QueueTeamID,
		ChannelID: input.ChannelID, ServiceStatus: input.ServiceStatus, Kinds: input.Kinds,
		Search: input.Search, SearchRange: input.SearchRange,
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
		cursor.Partition != input.Partition ||
		cursor.Scope != input.Scope || cursor.CustomerView != input.CustomerView || cursor.AssigneeIdentityID != input.AssigneeIdentityID ||
		cursor.QueueFilter != input.QueueFilter || cursor.QueueTeamID != input.QueueTeamID ||
		cursor.ChannelID != input.ChannelID || cursor.ServiceStatus != input.ServiceStatus || !slices.Equal(cursor.Kinds, input.Kinds) ||
		cursor.Search != input.Search || cursor.SearchRange != input.SearchRange ||
		!common.ValidUUID(cursor.ID) {
		return nil, ErrCursorInvalid
	}
	// 数据库 UUID 返回小写，统一比较与锚点匹配所用的编号。
	cursor.ID = strings.ToLower(cursor.ID)
	return &cursor, nil
}
