//go:build !server

package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/runforyou-ai/cervi/internal/apiproxy"
	"github.com/runforyou-ai/cervi/internal/appservice"
	appservicenative "github.com/runforyou-ai/cervi/internal/appservice/native"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// deviceRegistrar 把本机注册为企业设备并执行派发给本机的运行；不注册设备的原生平台为空。
type deviceRegistrar interface {
	appservice.LocalDeviceReporter
	// Start 开始注册与执行循环。
	Start()
	// Stop 结束注册与执行循环并等待其退出。
	Stop()
}

// applicationServices 创建原生端使用的远程应用服务和本机设备注册器。
func applicationServices(
	appStorage nativeStorage,
	nativeLocaleUpdater appservice.NativeLocaleUpdater,
	notification appservice.NativeNotification,
	unreadIndicator appservice.UnreadIndicator,
) ([]application.Service, deviceRegistrar, error) {
	sessions, err := clientsession.NewManager(context.Background(), appStorage)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize client session: %w", err)
	}
	backend, err := apiproxy.NewBackend(appStorage, sessions, func(name string, data any) {
		application.Get().Event.Emit(name, data)
	}, func(ctx context.Context) string {
		// 绑定调用携带发起窗口，窗口刷新后重新连接据此关闭原有事件流。
		window, ok := ctx.Value(application.WindowKey).(application.Window)
		if !ok || window == nil {
			return ""
		}
		return strconv.FormatUint(uint64(window.ID()), 10)
	})
	if err != nil {
		return nil, nil, fmt.Errorf("initialize remote application backend: %w", err)
	}
	options := []appservice.Option{
		appservice.WithImageSelector(appservicenative.NewImageSelector()),
		appservice.WithNativeLocaleUpdater(nativeLocaleUpdater),
		appservice.WithNativeNotification(notification),
		appservice.WithUnreadIndicator(unreadIndicator),
		appservice.WithConversationWindowOpener(appservicenative.NewConversationWindowOpener()),
	}
	registrar := newDeviceRegistrar(appStorage, backend, sessions)
	if registrar != nil {
		options = append(options, appservice.WithLocalDevice(registrar))
		// 为助理提供本机运行环境的平台同时开放本机环境管理。
		if manager, ok := registrar.(appservice.LocalEnvironmentManager); ok {
			options = append(options, appservice.WithLocalEnvironment(manager))
		}
	}
	service := appservice.New(backend, options...)
	return []application.Service{
		application.NewServiceWithOptions(service, application.ServiceOptions{
			MarshalError: appservice.MarshalError,
		}),
	}, registrar, nil
}
