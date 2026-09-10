//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	businesssystemaction "github.com/runforyou-ai/cervi/internal/actions/businesssystem"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ListBusinessSystems 返回当前企业配置的业务系统。
func (o *directOperations) ListBusinessSystems(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (BusinessSystemList, error) {
	records, err := o.listBusinessSystems.Execute(ctx, identity)
	if err != nil {
		return BusinessSystemList{}, o.businessSystemError(ctx, meta, err, cervii18n.ErrorBusinessSystemListFailed, identity.Organization.ID)
	}
	businessSystems := make([]BusinessSystem, 0, len(records))
	for _, record := range records {
		businessSystems = append(businessSystems, businessSystemFromAction(record))
	}
	return BusinessSystemList{BusinessSystems: businessSystems}, nil
}

// GetBusinessSystem 返回当前企业中的业务系统详情。
func (o *directOperations) GetBusinessSystem(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, businessSystemID string) (BusinessSystem, error) {
	record, err := o.getBusinessSystem.Execute(ctx, identity, businessSystemID)
	if err != nil {
		return BusinessSystem{}, o.businessSystemError(
			ctx, meta, err, cervii18n.ErrorBusinessSystemReadFailed, identity.Organization.ID,
			"business_system_id", businessSystemID,
		)
	}
	return businessSystemFromAction(*record), nil
}

// CreateBusinessSystem 创建业务系统。
func (o *directOperations) CreateBusinessSystem(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input BusinessSystemInput) (BusinessSystem, error) {
	record, err := o.createBusinessSystem.Execute(ctx, identity, businessSystemInput(input))
	if err != nil {
		return BusinessSystem{}, o.businessSystemMutationError(
			ctx, meta, err, cervii18n.ErrorBusinessSystemCreateFailed, identity.Organization.ID,
		)
	}
	slog.Info(
		"业务系统创建成功",
		"organization_id", identity.Organization.ID,
		"business_system_id", record.ID,
		"enabled", record.Enabled,
	)
	return businessSystemFromAction(*record), nil
}

// UpdateBusinessSystem 修改业务系统。
func (o *directOperations) UpdateBusinessSystem(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, businessSystemID string, input BusinessSystemInput) (BusinessSystem, error) {
	record, err := o.updateBusinessSystem.Execute(ctx, identity, businessSystemID, businessSystemInput(input))
	if err != nil {
		return BusinessSystem{}, o.businessSystemMutationError(
			ctx, meta, err, cervii18n.ErrorBusinessSystemUpdateFailed, identity.Organization.ID,
			"business_system_id", businessSystemID,
		)
	}
	slog.Info(
		"业务系统保存成功",
		"organization_id", identity.Organization.ID,
		"business_system_id", record.ID,
		"enabled", record.Enabled,
	)
	return businessSystemFromAction(*record), nil
}

// DeleteBusinessSystem 删除业务系统。
func (o *directOperations) DeleteBusinessSystem(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, businessSystemID string) error {
	if err := o.deleteBusinessSystem.Execute(ctx, identity, businessSystemID); err != nil {
		return o.businessSystemError(
			ctx, meta, err, cervii18n.ErrorBusinessSystemDeleteFailed, identity.Organization.ID,
			"business_system_id", businessSystemID,
		)
	}
	slog.Info("业务系统删除成功", "organization_id", identity.Organization.ID, "business_system_id", businessSystemID)
	return nil
}

// businessSystemMutationError 转换业务系统写入错误。
func (o *directOperations) businessSystemMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		// 映射业务系统校验错误。
		keys := map[common.FieldCode]cervii18n.Key{
			businesssystemaction.ValidationNameRequired:       cervii18n.FieldBusinessSystemNameRequired,
			businesssystemaction.ValidationNameTooLong:        cervii18n.FieldBusinessSystemNameTooLong,
			businesssystemaction.ValidationNameDuplicate:      cervii18n.FieldBusinessSystemNameDuplicate,
			businesssystemaction.ValidationDescriptionTooLong: cervii18n.FieldBusinessSystemDescriptionTooLong,
			businesssystemaction.ValidationURLRequired:        cervii18n.FieldBusinessSystemURLRequired,
			businesssystemaction.ValidationURLInvalid:         cervii18n.FieldHTTPURLInvalid,
			businesssystemaction.ValidationURLTooLong:         cervii18n.FieldBusinessSystemURLTooLong,
		}
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, keys))
	}
	return o.businessSystemError(ctx, meta, err, failureKey, organizationID, attributes...)
}

// businessSystemError 转换业务系统操作错误。
func (o *directOperations) businessSystemError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, businesssystemaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorBusinessSystemNotFound)
	}
	logAttributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	slog.Warn("业务系统操作失败", append(logAttributes, attributes...)...)
	return FailedError(meta, failureKey)
}

// businessSystemInput 转换业务系统输入。
func businessSystemInput(input BusinessSystemInput) businesssystemaction.Input {
	return businesssystemaction.Input{
		Name: input.Name, Description: input.Description, URL: input.URL, Enabled: input.Enabled,
	}
}

// businessSystemFromAction 转换业务系统输出。
func businessSystemFromAction(input businesssystemaction.Record) BusinessSystem {
	return BusinessSystem{
		ID: input.ID, Name: input.Name, Description: input.Description, URL: input.URL, Enabled: input.Enabled,
		CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt,
	}
}
