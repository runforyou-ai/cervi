//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// GetS3Setting 返回当前企业的对象存储设置。
func (b *DirectBackend) GetS3Setting(ctx context.Context, meta RequestMeta) (S3Setting, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return S3Setting{}, err
	}
	setting, err := b.getS3Setting.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return S3Setting{}, ctx.Err()
		}
		slog.Warn("读取对象存储设置失败", "organization_id", identity.Organization.ID, "error", err)
		return S3Setting{}, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorS3SettingReadFailed, nil)
	}
	return s3SettingFromAction(setting), nil
}

// SaveS3Setting 保存当前企业的对象存储设置。
func (b *DirectBackend) SaveS3Setting(ctx context.Context, meta RequestMeta, input S3Setting) (S3Setting, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return S3Setting{}, err
	}
	setting, err := b.saveS3Setting.Execute(ctx, identity, s3SettingToAction(input))
	if err != nil {
		return S3Setting{}, b.s3SettingError(ctx, meta, err, cervii18n.ErrorS3SettingSaveFailed)
	}
	slog.Info("对象存储设置保存成功", "organization_id", identity.Organization.ID, "provider", setting.Provider, "enabled", setting.Enabled)
	return s3SettingFromAction(setting), nil
}

// TestS3Setting 测试对象存储连接。
func (b *DirectBackend) TestS3Setting(ctx context.Context, meta RequestMeta, input S3Setting) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.testS3Setting.Execute(ctx, s3SettingToAction(input)); err != nil {
		return b.s3SettingError(ctx, meta, err, cervii18n.ErrorS3ConnectionTestFailed)
	}
	slog.Info("对象存储连接测试成功", "organization_id", identity.Organization.ID, "provider", input.Provider)
	return nil
}

func (b *DirectBackend) s3SettingError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return localizedError(meta, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, s3SettingFieldKeys(validationError.Fields))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return localizedError(meta, http.StatusUnauthorized, "AUTH_REQUIRED", cervii18n.ErrorAuthenticationRequired, nil)
	}
	if errors.Is(err, settingaction.ErrS3ConnectionFailed) {
		return localizedError(meta, http.StatusUnprocessableEntity, "S3_CONNECTION_FAILED", cervii18n.ErrorS3ConnectionTestFailed, nil)
	}
	slog.Warn("对象存储操作失败", "failure", failureKey, "error", err)
	return localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", failureKey, nil)
}

func s3SettingToAction(input S3Setting) settingaction.S3Setting {
	return settingaction.S3Setting{
		Enabled: input.Enabled, Provider: domain.StorageProvider(input.Provider), Endpoint: input.Endpoint, Region: input.Region,
		Bucket: input.Bucket, AccessKeyID: input.AccessKeyID, SecretAccessKey: input.SecretAccessKey, ForcePathStyle: input.ForcePathStyle,
	}
}

func s3SettingFromAction(input settingaction.S3Setting) S3Setting {
	return S3Setting{
		Enabled: input.Enabled, Provider: StorageProvider(input.Provider), Endpoint: input.Endpoint, Region: input.Region,
		Bucket: input.Bucket, AccessKeyID: input.AccessKeyID, SecretAccessKey: input.SecretAccessKey, ForcePathStyle: input.ForcePathStyle,
	}
}

func s3SettingFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		settingaction.ValidationEndpointRequired: cervii18n.FieldS3EndpointRequired, settingaction.ValidationEndpointInvalid: cervii18n.FieldS3EndpointInvalid,
		settingaction.ValidationProviderInvalid: cervii18n.FieldS3ProviderInvalid, settingaction.ValidationRegionRequired: cervii18n.FieldS3RegionRequired,
		settingaction.ValidationBucketRequired: cervii18n.FieldS3BucketRequired, settingaction.ValidationAccessKeyIDRequired: cervii18n.FieldS3AccessKeyIDRequired,
		settingaction.ValidationSecretAccessKeyRequired: cervii18n.FieldS3SecretAccessKeyRequired,
	}
	return translateValidationFields(fields, keys)
}
