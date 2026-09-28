//go:build server

package direct

import (
	"context"
	"errors"
	"log/slog"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// UpdateOrganization 修改当前工作区的名称。
func (o *directOperations) UpdateOrganization(ctx context.Context, meta appservice.RequestMeta, identity *servermodels.Identity, input appservice.OrganizationInput) (appservice.Organization, error) {
	organization, err := o.updateOrganization.Execute(ctx, identity, input.Name)
	if err != nil {
		return appservice.Organization{}, o.organizationMutationError(ctx, meta, err, cervii18n.ErrorOrganizationUpdateFailed, identity.Organization.ID)
	}
	slog.Info("工作区通用设置更新成功", "organization_id", organization.ID)
	return organizationFromModel(*organization), nil
}

// organizationMutationError 转换工作区设置写入错误。
func (o *directOperations) organizationMutationError(ctx context.Context, meta appservice.RequestMeta, err error, failureKey cervii18n.Key, organizationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return appservice.InvalidError(meta, cervii18n.ErrorValidationFailed, workspaceFieldKeys(validationError.Fields))
	}
	if errors.Is(err, identityaction.ErrInvalid) {
		return appservice.SessionError(meta, appservice.SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	slog.Warn("工作区设置操作失败", "organization_id", organizationID, "failure", failureKey, "error", err)
	return appservice.FailedError(meta, failureKey)
}
