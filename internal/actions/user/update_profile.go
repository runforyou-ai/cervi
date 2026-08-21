//go:build server

package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// ErrAvatarFileNotFound 表示头像文件不可关联。
var ErrAvatarFileNotFound = errors.New("avatar file not found")

// UpdateProfileAction 修改当前用户的个人资料。
type UpdateProfileAction struct {
	db *bun.DB
}

// NewUpdateProfileAction 创建个人资料修改操作。
func NewUpdateProfileAction(db *bun.DB) *UpdateProfileAction {
	return &UpdateProfileAction{db: db}
}

// Execute 校验并更新当前用户的姓名、邮箱和头像关联。
func (a *UpdateProfileAction) Execute(ctx context.Context, identity *servermodels.Identity, input ProfileInput) (*servermodels.User, error) {
	input, fields := normalizeProfileInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if identity == nil ||
		!common.ValidUUID(identity.Organization.ID) ||
		!common.ValidUUID(identity.User.ID) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return nil, common.ErrIdentityInvalid
	}

	user := &servermodels.User{}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var previousAvatarFileID *string
		query := tx.NewUpdate().
			Model(user).
			Set("display_name = ?", input.DisplayName).
			Set("email = ?", input.Email).
			Set("updated_at = now()")
		if input.AvatarFileID != "" {
			if !common.ValidUUID(input.AvatarFileID) {
				return ErrAvatarFileNotFound
			}
			currentUser := &servermodels.User{}
			if err := tx.NewSelect().Model(currentUser).
				Column("avatar_file_id").
				Where("u.id = ?", identity.User.ID).
				Where("u.organization_id = ?", identity.Organization.ID).
				For("UPDATE").
				Scan(ctx); errors.Is(err, sql.ErrNoRows) {
				return common.ErrIdentityInvalid
			} else if err != nil {
				return err
			}
			previousAvatarFileID = currentUser.AvatarFileID

			file := &servermodels.File{}
			if err := tx.NewSelect().Model(file).
				Column("id", "status", "expires_at").
				Where("f.id = ?", input.AvatarFileID).
				Where("f.organization_id = ?", identity.Organization.ID).
				Where("f.purpose = ?", domain.FilePurposeUserAvatar).
				For("UPDATE").
				Scan(ctx); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrAvatarFileNotFound
				}
				return err
			}
			sameAvatar := previousAvatarFileID != nil && *previousAvatarFileID == file.ID
			if file.Status == string(domain.FileStatusUploaded) {
				if file.ExpiresAt == nil || !file.ExpiresAt.After(time.Now().UTC()) {
					return ErrAvatarFileNotFound
				}
				if _, err := tx.NewUpdate().Model((*servermodels.File)(nil)).
					Set("status = ?", domain.FileStatusActive).
					Set("expires_at = NULL").
					Set("updated_at = now()").
					Where("id = ?", file.ID).
					Where("status = ?", domain.FileStatusUploaded).
					Exec(ctx); err != nil {
					return err
				}
			} else if file.Status != string(domain.FileStatusActive) || !sameAvatar {
				return ErrAvatarFileNotFound
			}
			query = query.Set("avatar_file_id = ?", file.ID)
		}
		result, err := query.
			Where("u.id = ?", identity.User.ID).
			Where("u.organization_id = ?", identity.Organization.ID).
			Returning("id, organization_id, email, display_name, role_id, status, locale, time_zone, work_status, avatar_file_id").
			Exec(ctx)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return common.ErrIdentityInvalid
		}
		if previousAvatarFileID != nil && *previousAvatarFileID != input.AvatarFileID {
			if _, err := tx.NewUpdate().Model((*servermodels.File)(nil)).
				Set("status = ?", domain.FileStatusDeleting).
				Set("expires_at = now()").
				Set("updated_at = now()").
				Where("id = ?", *previousAvatarFileID).
				Where("organization_id = ?", identity.Organization.ID).
				Where("status = ?", domain.FileStatusActive).
				Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	if isUniqueViolation(err) {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"email": ValidationEmailDuplicate}}
	}
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	return user, nil
}

// isUniqueViolation 判断 PostgreSQL 错误是否为唯一约束冲突。
func isUniqueViolation(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.Field('C') == "23505"
}
