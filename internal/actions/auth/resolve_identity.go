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

var (
	// ErrIdentityNotFound 表示会话令牌无效、已过期或对应账号已停用。
	ErrIdentityNotFound = errors.New("session not found or account inactive")
	// ErrMembershipNotFound 表示账号在目标工作区没有有效的成员身份。
	ErrMembershipNotFound = errors.New("account is not an active member of the workspace")
)

// ResolveAccountQuery 解析登录会话令牌对应的账号。
type ResolveAccountQuery struct {
	db *bun.DB
}

// NewResolveAccountQuery 创建登录账号查询。
func NewResolveAccountQuery(db *bun.DB) *ResolveAccountQuery {
	return &ResolveAccountQuery{db: db}
}

// Execute 返回有效会话令牌对应的账号和会话。
func (q *ResolveAccountQuery) Execute(ctx context.Context, value string) (*servermodels.AccountIdentity, error) {
	return resolveAccount(ctx, q.db, value)
}

// ResolveIdentityQuery 解析登录会话在目标工作区中的成员身份。
type ResolveIdentityQuery struct {
	db *bun.DB
}

// NewResolveIdentityQuery 创建工作区成员身份查询。
func NewResolveIdentityQuery(db *bun.DB) *ResolveIdentityQuery {
	return &ResolveIdentityQuery{db: db}
}

// Execute 返回会话令牌对应账号在目标工作区中的有效成员身份。
func (q *ResolveIdentityQuery) Execute(ctx context.Context, organizationID string, value string) (*servermodels.Identity, error) {
	account, err := resolveAccount(ctx, q.db, value)
	if err != nil {
		return nil, err
	}
	return ResolveMember(ctx, q.db, account, organizationID)
}

// resolveAccount 返回有效会话令牌对应的账号；令牌无效、已过期或账号停用时返回 ErrIdentityNotFound。
func resolveAccount(ctx context.Context, db bun.IDB, value string) (*servermodels.AccountIdentity, error) {
	if value == "" {
		return nil, ErrIdentityNotFound
	}
	identity := &servermodels.AccountIdentity{}
	err := db.NewRaw(`
		SELECT
			acc.id::text, acc.email, acc.email_verified_at, acc.display_name, acc.locale, acc.time_zone, acc.status, acc.is_deployment_admin,
			acs.id::text, acs.account_id::text, acs.expires_at
		FROM account_sessions AS acs
		JOIN accounts AS acc ON acc.id = acs.account_id
		WHERE acs.token_hash = ?
		  AND acs.expires_at > now()
	`, token.Hash(value)).Scan(ctx,
		&identity.Account.ID, &identity.Account.Email, &identity.Account.EmailVerifiedAt, &identity.Account.DisplayName,
		&identity.Account.Locale, &identity.Account.TimeZone, &identity.Account.Status, &identity.Account.IsDeploymentAdmin,
		&identity.Session.ID, &identity.Session.AccountID, &identity.Session.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIdentityNotFound
	}
	if err != nil {
		return nil, err
	}
	if identity.Account.Status != string(domain.AccountStatusActive) {
		return nil, ErrIdentityNotFound
	}
	return identity, nil
}

// ResolveMember 返回账号在目标工作区中的有效成员身份；没有成员身份或成员已停用时返回 ErrMembershipNotFound。
func ResolveMember(ctx context.Context, db bun.IDB, account *servermodels.AccountIdentity, organizationID string) (*servermodels.Identity, error) {
	if !common.ValidUUID(organizationID) {
		return nil, ErrMembershipNotFound
	}
	identity := &servermodels.Identity{Account: account.Account, Session: account.Session}
	err := db.NewRaw(`
		SELECT
			o.id::text, o.name, o.slug,
			u.id::text, u.identity_id::text, u.organization_id::text, u.account_id::text, u.status,
			u.translation_language, u.message_notifications_enabled, u.role_id::text,
			oi.id::text, oi.organization_id::text, oi.type, oi.display_name, oi.avatar_file_id::text, oi.handles_customers, oi.work_status
		FROM users AS u
		JOIN organization_identities AS oi ON oi.id = u.identity_id AND oi.organization_id = u.organization_id AND oi.type = ?
		JOIN organizations AS o ON o.id = u.organization_id
		WHERE u.organization_id = ?
		  AND u.account_id = ?
	`, domain.OrganizationIdentityTypeUser, organizationID, account.Account.ID).Scan(ctx,
		&identity.Organization.ID, &identity.Organization.Name, &identity.Organization.Slug,
		&identity.User.ID, &identity.User.IdentityID, &identity.User.OrganizationID, &identity.User.AccountID, &identity.User.Status,
		&identity.User.TranslationLanguage, &identity.User.MessageNotificationsEnabled, &identity.User.RoleID,
		&identity.OrganizationIdentity.ID, &identity.OrganizationIdentity.OrganizationID, &identity.OrganizationIdentity.Type,
		&identity.OrganizationIdentity.DisplayName, &identity.OrganizationIdentity.AvatarFileID,
		&identity.OrganizationIdentity.HandlesCustomers, &identity.OrganizationIdentity.WorkStatus,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMembershipNotFound
	}
	if err != nil {
		return nil, err
	}
	if identity.User.Status != string(domain.UserStatusActive) {
		return nil, ErrMembershipNotFound
	}
	return identity, nil
}
