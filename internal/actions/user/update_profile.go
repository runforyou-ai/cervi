//go:build server

package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// UpdateProfileAction 修改当前用户的个人资料。
type UpdateProfileAction struct {
	db *bun.DB
}

// NewUpdateProfileAction 创建个人资料修改操作。
func NewUpdateProfileAction(db *bun.DB) *UpdateProfileAction {
	return &UpdateProfileAction{db: db}
}

// Execute 校验并更新当前用户的姓名和邮箱。
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
	result, err := a.db.NewUpdate().
		Model(user).
		Set("display_name = ?", input.DisplayName).
		Set("email = ?", input.Email).
		Set("updated_at = now()").
		Where("u.id = ?", identity.User.ID).
		Where("u.organization_id = ?", identity.Organization.ID).
		Returning("id, organization_id, email, display_name, role, status").
		Exec(ctx)
	if isUniqueViolation(err) {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"email": ValidationEmailDuplicate}}
	}
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read updated profile count: %w", err)
	}
	if rows == 0 {
		return nil, common.ErrIdentityInvalid
	}
	return user, nil
}

// isUniqueViolation 判断 PostgreSQL 错误是否为唯一约束冲突。
func isUniqueViolation(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.Field('C') == "23505"
}
