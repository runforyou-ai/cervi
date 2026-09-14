//go:build server

package identity

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateUserIdentity 更新真人企业身份，登录态可见字段实际变化时同句推进用户身份资料版本；调用方需先锁定对应用户账号。
func UpdateUserIdentity(ctx context.Context, db bun.IDB, organizationID, identityID string, query *bun.UpdateQuery) error {
	query = query.
		Where("oi.organization_id = ? AND oi.id = ? AND oi.type = ?", organizationID, identityID, domain.OrganizationIdentityTypeUser).
		Returning("new.organization_id, new.id, (old.display_name, old.avatar_file_id, old.role_id, old.work_status) IS DISTINCT FROM (new.display_name, new.avatar_file_id, new.role_id, new.work_status) AS changed")
	_, err := db.NewUpdate().With("identity_change", query).
		Model((*servermodels.User)(nil)).
		Set("profile_version = u.profile_version + 1").
		Where("(u.organization_id, u.identity_id) IN (SELECT organization_id, id FROM identity_change WHERE changed)").
		Exec(ctx)
	return err
}
