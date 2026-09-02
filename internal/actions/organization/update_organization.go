//go:build server

package organization

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateOrganizationAction 修改企业通用设置。
type UpdateOrganizationAction struct {
	db *bun.DB
}

// NewUpdateOrganizationAction 创建企业通用设置修改操作。
func NewUpdateOrganizationAction(db *bun.DB) *UpdateOrganizationAction {
	return &UpdateOrganizationAction{db: db}
}

// Execute 校验并修改当前用户所属企业的通用设置。
func (a *UpdateOrganizationAction) Execute(ctx context.Context, identity *servermodels.Identity, input Input) (*servermodels.Organization, error) {
	// 归一化并校验企业通用设置。
	input.Name = strings.TrimSpace(input.Name)
	fields := make(map[string]ValidationCode)
	if input.Name == "" {
		fields["name"] = ValidationNameRequired
	} else if utf8.RuneCountInString(input.Name) > domain.OrganizationNameMaxLength {
		fields["name"] = ValidationNameTooLong
	}
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var organization *servermodels.Organization
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		organization = &servermodels.Organization{
			ID:                identity.Organization.ID,
			Name:              input.Name,
			AllowArbitraryURL: input.AllowArbitraryURL,
		}
		_, err := tx.NewUpdate().
			Model(organization).
			Column("name", "allow_arbitrary_url").
			Set("updated_at = now()").
			WherePK().
			Returning("*").
			Exec(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}
	return organization, nil
}
