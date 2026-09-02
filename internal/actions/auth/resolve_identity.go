//go:build server

package auth

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/common/token"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ErrIdentityNotFound 表示令牌无效、已过期或对应账号不可用。
var ErrIdentityNotFound = errors.New("token identity not found or inactive")

// ResolveIdentityQuery 解析当前登录令牌对应的身份。
type ResolveIdentityQuery struct {
	db *bun.DB
}

// NewResolveIdentityQuery 创建登录身份查询。
func NewResolveIdentityQuery(db *bun.DB) *ResolveIdentityQuery {
	return &ResolveIdentityQuery{db: db}
}

// Execute 返回有效令牌对应的用户身份。
func (q *ResolveIdentityQuery) Execute(ctx context.Context, organizationID string, value string) (*servermodels.Identity, error) {
	return resolveIdentity(ctx, q.db, organizationID, value)
}

// resolveIdentity 返回有效令牌对应的用户身份；令牌无效或账号停用时返回 ErrIdentityNotFound。
func resolveIdentity(ctx context.Context, db bun.IDB, organizationID string, value string) (*servermodels.Identity, error) {
	if !common.ValidUUID(organizationID) {
		return nil, ErrIdentityNotFound
	}
	identity := &servermodels.Identity{}
	err := db.NewRaw(`
		SELECT
			o.id::text,
			o.name,
			u.id::text,
			u.identity_id::text,
			u.organization_id::text,
			u.email,
			u.status,
			u.locale,
			u.time_zone,
			u.message_notifications_enabled,
			u.workspace_tabs_enabled,
			oi.id::text,
			oi.organization_id::text,
			oi.type,
			oi.role_id::text,
			oi.display_name,
			oi.avatar_file_id::text,
			oi.work_status
		FROM tokens AS token
		JOIN users AS u ON u.id = token.user_id
		JOIN organization_identities AS oi ON oi.id = u.identity_id AND oi.organization_id = u.organization_id AND oi.type = ?
		JOIN organizations AS o ON o.id = u.organization_id
		WHERE u.organization_id = ?
		  AND token.token_hash = ?
		  AND token.expires_at > now()
		LIMIT 1
	`, domain.OrganizationIdentityTypeUser, organizationID, token.Hash(value)).Scan(
		ctx,
		&identity.Organization.ID,
		&identity.Organization.Name,
		&identity.User.ID,
		&identity.User.IdentityID,
		&identity.User.OrganizationID,
		&identity.User.Email,
		&identity.User.Status,
		&identity.User.Locale,
		&identity.User.TimeZone,
		&identity.User.MessageNotificationsEnabled,
		&identity.User.WorkspaceTabsEnabled,
		&identity.OrganizationIdentity.ID,
		&identity.OrganizationIdentity.OrganizationID,
		&identity.OrganizationIdentity.Type,
		&identity.OrganizationIdentity.RoleID,
		&identity.OrganizationIdentity.DisplayName,
		&identity.OrganizationIdentity.AvatarFileID,
		&identity.OrganizationIdentity.WorkStatus,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIdentityNotFound
	}
	if err != nil {
		return nil, err
	}
	if identity.User.Status != string(domain.UserStatusActive) {
		return nil, ErrIdentityNotFound
	}
	return identity, nil
}
