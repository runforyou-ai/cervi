//go:build !server && !ios && !android

package main

import (
	"context"
	"log/slog"

	"github.com/runforyou-ai/cervi/internal/apiproxy"
	"github.com/runforyou-ai/cervi/internal/appservice"
	appservicenative "github.com/runforyou-ai/cervi/internal/appservice/native"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/runforyou-ai/cervi/internal/devicehost"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

// nativeStorage 组合桌面端连接、登录凭据、设备注册与本机工作区存储能力。
type nativeStorage interface {
	apiproxy.Store
	clientsession.Store
	devicehost.Store
	devicehost.WorkspaceStore
	devicehost.LocalWorkspaceStore
}

// desktopDevice 组合桌面端本机设备注册、Agent 运行执行循环与本机工作区管理。
type desktopDevice struct {
	*devicehost.Registrar
	worker     *devicehost.Worker
	workspaces *devicehost.Workspaces
}

// Start 开始设备注册与执行循环。
func (d *desktopDevice) Start() {
	d.Registrar.Start()
	d.worker.Start()
}

// Stop 结束执行循环与设备注册并等待其退出。
func (d *desktopDevice) Stop() {
	d.worker.Stop()
	d.Registrar.Stop()
}

// AddLocalWorkspace 让用户选择本机目录并注册为本设备的工作区。
func (d *desktopDevice) AddLocalWorkspace(ctx context.Context, meta appservice.RequestMeta) (appservice.DeviceWorkspace, error) {
	return d.workspaces.AddLocalWorkspace(ctx, meta)
}

// newDeviceRegistrar 创建桌面端本机设备注册、执行循环与工作区管理；本机运行时创建失败时不注册设备。
func newDeviceRegistrar(appStorage nativeStorage, backend *apiproxy.Backend, sessions *clientsession.Manager) deviceRegistrar {
	registrar := devicehost.New(appStorage, backend, sessions)
	if registrar == nil {
		return nil
	}
	runtime, err := agentruntime.New()
	if err != nil {
		slog.Error("创建本机 Agent 运行时失败，本机设备不注册", "error", err)
		return nil
	}
	return &desktopDevice{
		Registrar:  registrar,
		worker:     devicehost.NewWorker(registrar, appStorage, backend, runtime),
		workspaces: devicehost.NewWorkspaces(registrar, appStorage, backend, appservicenative.SelectWorkspaceDirectory),
	}
}
