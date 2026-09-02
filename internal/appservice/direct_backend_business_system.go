//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	businesssystemaction "github.com/runforyou-ai/cervi/internal/actions/businesssystem"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// ListBusinessSystems 返回当前企业配置的业务系统。
func (b *DirectBackend) ListBusinessSystems(ctx context.Context, meta RequestMeta) (BusinessSystemList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return BusinessSystemList{}, err
	}
	records, err := b.listBusinessSystems.Execute(ctx, identity)
	if err != nil {
		return BusinessSystemList{}, b.businessSystemError(ctx, meta, err, cervii18n.ErrorBusinessSystemListFailed, identity.Organization.ID)
	}
	businessSystems := make([]BusinessSystem, 0, len(records))
	for _, record := range records {
		businessSystems = append(businessSystems, businessSystemFromAction(record))
	}
	return BusinessSystemList{BusinessSystems: businessSystems}, nil
}

// GetBusinessSystem 返回当前企业中的业务系统详情。
func (b *DirectBackend) GetBusinessSystem(ctx context.Context, meta RequestMeta, businessSystemID string) (BusinessSystem, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return BusinessSystem{}, err
	}
	record, err := b.getBusinessSystem.Execute(ctx, identity, businessSystemID)
	if err != nil {
		return BusinessSystem{}, b.businessSystemError(
			ctx, meta, err, cervii18n.ErrorBusinessSystemReadFailed, identity.Organization.ID,
			"business_system_id", businessSystemID,
		)
	}
	return businessSystemFromAction(*record), nil
}

// CreateBusinessSystem 创建业务系统。
func (b *DirectBackend) CreateBusinessSystem(ctx context.Context, meta RequestMeta, input BusinessSystemInput) (BusinessSystem, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return BusinessSystem{}, err
	}
	record, err := b.createBusinessSystem.Execute(ctx, identity, businessSystemInput(input))
	if err != nil {
		return BusinessSystem{}, b.businessSystemMutationError(
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
func (b *DirectBackend) UpdateBusinessSystem(ctx context.Context, meta RequestMeta, businessSystemID string, input BusinessSystemInput) (BusinessSystem, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return BusinessSystem{}, err
	}
	record, err := b.updateBusinessSystem.Execute(ctx, identity, businessSystemID, businessSystemInput(input))
	if err != nil {
		return BusinessSystem{}, b.businessSystemMutationError(
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
func (b *DirectBackend) DeleteBusinessSystem(ctx context.Context, meta RequestMeta, businessSystemID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteBusinessSystem.Execute(ctx, identity, businessSystemID); err != nil {
		return b.businessSystemError(
			ctx, meta, err, cervii18n.ErrorBusinessSystemDeleteFailed, identity.Organization.ID,
			"business_system_id", businessSystemID,
		)
	}
	slog.Info("业务系统删除成功", "organization_id", identity.Organization.ID, "business_system_id", businessSystemID)
	return nil
}

// businessSystemMutationError 转换业务系统写入错误。
func (b *DirectBackend) businessSystemMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		// 映射业务系统校验错误。
		keys := map[common.FieldCode]cervii18n.Key{
			businesssystemaction.ValidationNameRequired:       cervii18n.FieldBusinessSystemNameRequired,
			businesssystemaction.ValidationNameTooLong:        cervii18n.FieldBusinessSystemNameTooLong,
			businesssystemaction.ValidationNameDuplicate:      cervii18n.FieldBusinessSystemNameDuplicate,
			businesssystemaction.ValidationDescriptionTooLong: cervii18n.FieldBusinessSystemDescriptionTooLong,
			businesssystemaction.ValidationURLRequired:        cervii18n.FieldBusinessSystemURLRequired,
			businesssystemaction.ValidationURLInvalid:         cervii18n.FieldBusinessSystemURLInvalid,
			businesssystemaction.ValidationURLTooLong:         cervii18n.FieldBusinessSystemURLTooLong,
		}
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, keys))
	}
	return b.businessSystemError(ctx, meta, err, failureKey, organizationID, attributes...)
}

// businessSystemError 转换业务系统操作错误。
func (b *DirectBackend) businessSystemError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
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
