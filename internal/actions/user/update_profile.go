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
func (a *UpdateProfileAction) Execute(ctx context.Context, identity *servermodels.Identity, input ProfileInput) (*servermodels.Identity, error) {
	input, fields := normalizeProfileInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var updatedIdentity *servermodels.Identity
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		var previousAvatarFileID *string
		identityQuery := tx.NewUpdate().
			Model((*servermodels.OrganizationIdentity)(nil)).
			Set("display_name = ?", input.DisplayName).
			Set("updated_at = now()")
		if input.AvatarFileID != "" {
			if !common.ValidUUID(input.AvatarFileID) {
				return ErrAvatarFileNotFound
			}
			currentIdentity := &servermodels.OrganizationIdentity{}
			if err := tx.NewSelect().Model(currentIdentity).
				Column("avatar_file_id").
				Where("oi.id = ?", identity.User.IdentityID).
				Where("oi.organization_id = ?", identity.Organization.ID).
				Where("oi.type = ?", domain.OrganizationIdentityTypeUser).
				For("UPDATE").
				Scan(ctx); errors.Is(err, sql.ErrNoRows) {
				return common.ErrIdentityInvalid
			} else if err != nil {
				return err
			}
			previousAvatarFileID = currentIdentity.AvatarFileID

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
			identityQuery = identityQuery.Set("avatar_file_id = ?", file.ID)
		}
		_, err := tx.NewUpdate().Model((*servermodels.User)(nil)).
			Set("email = ?", input.Email).
			Set("updated_at = now()").
			Where("u.id = ?", identity.User.ID).
			Where("u.organization_id = ?", identity.Organization.ID).
			Exec(ctx)
		if err != nil {
			return err
		}
		_, err = identityQuery.
			Where("oi.id = ?", identity.User.IdentityID).
			Where("oi.organization_id = ?", identity.Organization.ID).
			Where("oi.type = ?", domain.OrganizationIdentityTypeUser).
			Exec(ctx)
		if err != nil {
			return err
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
		updatedIdentity, err = loadCurrentIdentity(ctx, tx, identity.Organization, identity.User.ID)
		return err
	})
	if isUniqueViolation(err) {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"email": ValidationEmailDuplicate}}
	}
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	return updatedIdentity, nil
}

// isUniqueViolation 判断 PostgreSQL 错误是否为唯一约束冲突。
func isUniqueViolation(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.Field('C') == "23505"
}
