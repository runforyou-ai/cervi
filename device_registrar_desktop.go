//go:build !server && !ios && !android

package main

import (
	"github.com/runforyou-ai/cervi/internal/apiproxy"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/runforyou-ai/cervi/internal/devicehost"
)

// nativeStorage 组合桌面端连接、登录凭据和设备注册存储能力。
type nativeStorage interface {
	apiproxy.Store
	clientsession.Store
	devicehost.Store
}

// newDeviceRegistrar 创建桌面端本机设备注册器。
func newDeviceRegistrar(appStorage nativeStorage, backend *apiproxy.Backend, sessions *clientsession.Manager) deviceRegistrar {
	registrar := devicehost.New(appStorage, backend, sessions)
	if registrar == nil {
		return nil
	}
	return registrar
}
