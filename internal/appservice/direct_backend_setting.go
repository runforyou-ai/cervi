//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
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
		return S3Setting{}, FailedError(meta, cervii18n.ErrorS3SettingReadFailed)
	}
	return s3SettingFromAction(setting), nil
}

// SaveS3Setting 保存当前企业的对象存储设置。
func (b *DirectBackend) SaveS3Setting(ctx context.Context, meta RequestMeta, input S3SettingInput) (S3Setting, error) {
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
func (b *DirectBackend) TestS3Setting(ctx context.Context, meta RequestMeta, input S3SettingInput) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.testS3Setting.Execute(ctx, identity.Organization.ID, s3SettingToAction(input)); err != nil {
		return b.s3SettingError(ctx, meta, err, cervii18n.ErrorS3ConnectionTestFailed)
	}
	return nil
}

// s3SettingError 转换对象存储校验和操作错误。
func (b *DirectBackend) s3SettingError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, s3SettingFieldKeys(validationError.Fields))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if probeFailure, ok := settingaction.S3TestFailureOf(err); ok {
		stage, kind, _ := connectiontest.Details(err)
		slog.Warn("对象存储连接测试失败", "failure", failureKey, "probe_failure", probeFailure, "stage", stage, "kind", kind, "error", err)
		return UnavailableError(meta, s3TestFailureKey(probeFailure), nil)
	}
	if stage, kind, ok := connectiontest.Details(err); ok {
		slog.Warn("对象存储连接测试失败", "failure", failureKey, "stage", stage, "kind", kind, "error", err)
		return UnavailableError(meta, cervii18n.ErrorS3ConnectionTestFailed, nil)
	}
	slog.Warn("对象存储操作失败", "failure", failureKey, "error", err)
	return FailedError(meta, failureKey)
}

// s3TestFailureKey 返回对象存储探针失败能力对应的本地化文案。
func s3TestFailureKey(failure settingaction.S3TestFailure) cervii18n.Key {
	switch failure {
	case settingaction.S3TestFailureBucketAccess:
		return cervii18n.ErrorS3BucketAccessFailed
	case settingaction.S3TestFailureCORS:
		return cervii18n.ErrorS3CORSFailed
	case settingaction.S3TestFailureUpload:
		return cervii18n.ErrorS3UploadTestFailed
	case settingaction.S3TestFailurePublicAccess:
		return cervii18n.ErrorS3PublicAccessFailed
	case settingaction.S3TestFailureCleanup:
		return cervii18n.ErrorS3CleanupFailed
	default:
		return cervii18n.ErrorS3ConnectionTestFailed
	}
}

// s3SettingToAction 把对象存储配置输入转换为动作层配置。
func s3SettingToAction(input S3SettingInput) settingaction.S3Setting {
	return settingaction.S3Setting{
		Enabled: input.Enabled, Provider: domain.StorageProvider(input.Provider), Endpoint: input.Endpoint, PublicBaseURL: input.PublicBaseURL, Region: input.Region,
		Bucket: input.Bucket, AccessKeyID: input.AccessKeyID, SecretAccessKey: input.SecretAccessKey, ForcePathStyle: input.ForcePathStyle,
	}
}

// s3SettingFromAction 把动作层配置转换为应用契约。
func s3SettingFromAction(input settingaction.S3Setting) S3Setting {
	return S3Setting{
		Enabled: input.Enabled, Provider: StorageProvider(input.Provider), Endpoint: input.Endpoint, PublicBaseURL: input.PublicBaseURL, Region: input.Region,
		Bucket: input.Bucket, AccessKeyID: input.AccessKeyID, SecretAccessKey: input.SecretAccessKey, ForcePathStyle: input.ForcePathStyle,
	}
}

// s3SettingFieldKeys 把对象存储配置校验错误码映射为本地化文案键。
func s3SettingFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		settingaction.ValidationEndpointRequired: cervii18n.FieldS3EndpointRequired, settingaction.ValidationEndpointInvalid: cervii18n.FieldS3EndpointInvalid,
		settingaction.ValidationPublicBaseURLRequired: cervii18n.FieldS3PublicBaseURLRequired, settingaction.ValidationPublicBaseURLInvalid: cervii18n.FieldS3PublicBaseURLInvalid,
		settingaction.ValidationProviderInvalid: cervii18n.FieldS3ProviderInvalid, settingaction.ValidationRegionRequired: cervii18n.FieldS3RegionRequired,
		settingaction.ValidationBucketRequired: cervii18n.FieldS3BucketRequired, settingaction.ValidationAccessKeyIDRequired: cervii18n.FieldS3AccessKeyIDRequired,
		settingaction.ValidationSecretAccessKeyRequired: cervii18n.FieldS3SecretAccessKeyRequired,
	}
	return translateValidationFields(fields, keys)
}
