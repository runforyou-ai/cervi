//go:build server

// Package organization 实现工作区创建、通用设置修改和运营侧工作区查询。
package organization

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

// ValidationCode 标识工作区设置的校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationNameRequired ValidationCode = "ORGANIZATION_NAME_REQUIRED"
	ValidationNameTooLong  ValidationCode = "ORGANIZATION_NAME_TOO_LONG"
)

// ValidationError 表示工作区设置校验失败。
type ValidationError = common.FieldError

// UpdateOrganizationAction 修改工作区通用设置。
type UpdateOrganizationAction struct {
	db *bun.DB
}

// NewUpdateOrganizationAction 创建工作区通用设置修改操作。
func NewUpdateOrganizationAction(db *bun.DB) *UpdateOrganizationAction {
	return &UpdateOrganizationAction{db: db}
}

// Execute 校验并修改当前成员所在工作区的名称和标识。
func (a *UpdateOrganizationAction) Execute(ctx context.Context, identity *servermodels.Identity, input WorkspaceInput) (*servermodels.Organization, error) {
	input, fields := NormalizeWorkspaceInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var organization *servermodels.Organization
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		organization = &servermodels.Organization{
			ID:   identity.Organization.ID,
			Name: input.Name,
			Slug: input.Slug,
		}
		_, err := tx.NewUpdate().
			Model(organization).
			Column("name", "slug").
			Set("updated_at = now()").
			WherePK().
			Returning("*").
			Exec(ctx)
		if pgerr.UniqueViolationOn(err, "organizations_slug_unique") {
			return &ValidationError{Fields: map[string]ValidationCode{"slug": ValidationSlugTaken}}
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}
	return organization, nil
}
