//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	deviceaction "github.com/runforyou-ai/cervi/internal/actions/device"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// deviceOps 持有本机设备的 Action 和 Query。
type deviceOps struct {
	registerDevice      *deviceaction.RegisterDeviceAction
	listDevices         *deviceaction.ListDevicesQuery
	revokeDevice        *deviceaction.RevokeDeviceAction
	deviceAuthenticator *deviceaction.AuthenticateDeviceAction
}

// newDeviceOps 创建本机设备的业务实现依赖。
func newDeviceOps(db *bun.DB) deviceOps {
	return deviceOps{
		registerDevice:      deviceaction.NewRegisterDeviceAction(db),
		listDevices:         deviceaction.NewListDevicesQuery(db),
		revokeDevice:        deviceaction.NewRevokeDeviceAction(db),
		deviceAuthenticator: deviceaction.NewAuthenticateDeviceAction(db),
	}
}

// RegisterDevice 注册当前用户的本机设备。
func (o *directOperations) RegisterDevice(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input DeviceRegistrationInput) (Device, error) {
	record, err := o.registerDevice.Execute(ctx, identity, deviceaction.RegisterInput{
		InstallID: input.InstallID, Name: input.Name, Platform: domain.DevicePlatform(input.Platform),
	})
	if err != nil {
		return Device{}, o.deviceError(ctx, meta, err, cervii18n.ErrorDeviceRegisterFailed, identity.Organization.ID)
	}
	return deviceFromAction(*record), nil
}

// ListDevices 返回当前用户已注册的设备。
func (o *directOperations) ListDevices(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (DeviceList, error) {
	records, err := o.listDevices.Execute(ctx, identity)
	if err != nil {
		return DeviceList{}, o.deviceError(ctx, meta, err, cervii18n.ErrorDeviceListFailed, identity.Organization.ID)
	}
	devices := make([]Device, 0, len(records))
	for _, record := range records {
		devices = append(devices, deviceFromAction(record))
	}
	return DeviceList{Devices: devices}, nil
}

// RevokeDevice 撤销当前用户的设备。
func (o *directOperations) RevokeDevice(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, deviceID string) error {
	if err := o.revokeDevice.Execute(ctx, identity, deviceID); err != nil {
		return o.deviceError(ctx, meta, err, cervii18n.ErrorDeviceRevokeFailed, identity.Organization.ID, "device_id", deviceID)
	}
	return nil
}

// deviceError 转换本机设备操作错误。设备注册信息由客户端程序上报，校验失败按注册失败收敛并记录字段原因码。
func (o *directOperations) deviceError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, deviceaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorDeviceNotFound)
	}
	if errors.Is(err, chatstate.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	logAttributes := []any{"organization_id", organizationID, "failure", failureKey}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		logAttributes = append(logAttributes, "fields", validationError.Fields)
	} else {
		logAttributes = append(logAttributes, "error", err)
	}
	slog.Warn("设备操作失败", append(logAttributes, attributes...)...)
	return FailedError(meta, failureKey)
}

// deviceFromAction 转换设备输出。
func deviceFromAction(input deviceaction.Record) Device {
	return Device{
		ID: input.ID, Name: input.Name, Platform: DevicePlatform(input.Platform),
		CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt,
	}
}
