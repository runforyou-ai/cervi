//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// UpdateOrganization 修改当前工作区的名称和标识。
func (o *directOperations) UpdateOrganization(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input OrganizationInput) (Organization, error) {
	organization, err := o.updateOrganization.Execute(ctx, identity, organizationaction.WorkspaceInput{Name: input.Name, Slug: input.Slug})
	if err != nil {
		return Organization{}, o.organizationMutationError(ctx, meta, err, cervii18n.ErrorOrganizationUpdateFailed, identity.Organization.ID)
	}
	slog.Info("工作区通用设置更新成功", "organization_id", organization.ID)
	return organizationFromModel(*organization), nil
}

// organizationMutationError 转换工作区设置写入错误。
func (o *directOperations) organizationMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, workspaceFieldKeys(validationError.Fields))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	slog.Warn("工作区设置操作失败", "organization_id", organizationID, "failure", failureKey, "error", err)
	return FailedError(meta, failureKey)
}
