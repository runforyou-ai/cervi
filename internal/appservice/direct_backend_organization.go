//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// UpdateOrganization 修改企业名称。
func (b *DirectBackend) UpdateOrganization(ctx context.Context, meta RequestMeta, input OrganizationInput) (Organization, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Organization{}, err
	}
	organization, err := b.updateOrganization.Execute(ctx, identity, organizationaction.Input{Name: input.Name})
	if err != nil {
		if ctx.Err() != nil {
			return Organization{}, ctx.Err()
		}
		var validationError *common.FieldError
		if errors.As(err, &validationError) {
			return Organization{}, InvalidError(meta, cervii18n.ErrorValidationFailed, organizationFieldKeys(validationError.Fields))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return Organization{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("企业名称更新失败", "organization_id", identity.Organization.ID, "error", err)
		return Organization{}, FailedError(meta, cervii18n.ErrorOrganizationUpdateFailed)
	}
	slog.Info("企业名称更新成功", "organization_id", organization.ID)
	return organizationFromModel(*organization), nil
}

// organizationFieldKeys 把企业名称校验错误码映射为本地化文案键。
func organizationFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		organizationaction.ValidationNameRequired: cervii18n.FieldOrganizationNameRequired,
		organizationaction.ValidationNameTooLong:  cervii18n.FieldOrganizationNameTooLong,
	}
	return translateValidationFields(fields, keys)
}
