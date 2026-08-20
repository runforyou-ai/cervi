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
