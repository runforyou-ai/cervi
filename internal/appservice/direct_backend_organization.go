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

// UpdateOrganization 修改企业通用设置。
func (b *DirectBackend) UpdateOrganization(ctx context.Context, meta RequestMeta, input OrganizationInput) (Organization, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Organization{}, err
	}
	organization, err := b.updateOrganization.Execute(ctx, identity, organizationaction.Input{
		Name: input.Name, AllowArbitraryURL: input.AllowArbitraryURL,
	})
	if err != nil {
		return Organization{}, b.organizationMutationError(ctx, meta, err, cervii18n.ErrorOrganizationUpdateFailed, identity.Organization.ID)
	}
	slog.Info(
		"企业通用设置更新成功",
		"organization_id", organization.ID,
		"allow_arbitrary_url", organization.AllowArbitraryURL,
	)
	return organizationFromModel(*organization), nil
}

// organizationMutationError 转换企业设置写入错误。
func (b *DirectBackend) organizationMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		// 把企业通用设置校验错误码映射为本地化文案键。
		keys := map[common.FieldCode]cervii18n.Key{
			organizationaction.ValidationNameRequired: cervii18n.FieldOrganizationNameRequired,
			organizationaction.ValidationNameTooLong:  cervii18n.FieldOrganizationNameTooLong,
		}
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, keys))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	slog.Warn("企业设置操作失败", "organization_id", organizationID, "failure", failureKey, "error", err)
	return FailedError(meta, failureKey)
}
