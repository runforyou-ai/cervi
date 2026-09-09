//go:build server

package inbox

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

const inboxSortVersion = 1

// inboxCursorPoint 保存数据库原始活动时间，空时间使用独立的编号边界。
type inboxCursorPoint struct {
	ID             string     `json:"id" bun:"id"`
	LastActivityAt *time.Time `json:"lastActivityAt" bun:"last_activity_at"`
}

// inboxCursor 将排序边界绑定到当前企业、用户和规范化筛选。
type inboxCursor struct {
	inboxCursorPoint
	Version            int                      `json:"version"`
	OrganizationID     string                   `json:"organizationId"`
	UserID             string                   `json:"userId"`
	Scope              domain.InboxScope        `json:"scope"`
	CustomerView       domain.CustomerInboxView `json:"customerView"`
	AssigneeIdentityID string                   `json:"assigneeIdentityId"`
}

// encodeInboxCursor 无损编码活动时间，不依赖边界会话继续存在。
func encodeInboxCursor(identity *servermodels.Identity, input LoadInput, point inboxCursorPoint) (string, error) {
	data, err := json.Marshal(inboxCursor{
		inboxCursorPoint: point, Version: inboxSortVersion,
		OrganizationID: identity.Organization.ID, UserID: identity.User.ID,
		Scope: input.Scope, CustomerView: input.CustomerView, AssigneeIdentityID: input.AssigneeIdentityID,
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
		!common.ValidUUID(cursor.ID) {
		return nil, ErrCursorInvalid
	}
	return &cursor, nil
}
