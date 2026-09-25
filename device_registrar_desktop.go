//go:build !server && !ios && !android

package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/runforyou-ai/cervi/internal/apiproxy"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/runforyou-ai/cervi/internal/devicehost"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/toolchain"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// nativeStorage 组合桌面端连接、登录凭据与设备注册存储能力。
type nativeStorage interface {
	apiproxy.Store
	clientsession.Store
	devicehost.Store
}

// desktopDevice 组合桌面端本机设备注册、Agent 运行执行循环与运行环境。
type desktopDevice struct {
	*devicehost.Registrar
	worker    *devicehost.Worker
	toolchain *toolchain.Manager
}

// CurrentDevice 返回本机设备注册状态与 Agent 运行环境的准备状态。
func (d *desktopDevice) CurrentDevice(ctx context.Context, meta appservice.RequestMeta) (appservice.LocalDevice, error) {
	device, err := d.Registrar.CurrentDevice(ctx, meta)
	if err != nil {
		return device, err
	}
	status := d.toolchain.Status()
	local := appservice.LocalToolchain{State: appservice.LocalToolchainState(status.State)}
	if status.Failure != "" {
		failure := appservice.LocalToolchainFailure(status.Failure)
		local.Failure = &failure
	}
	device.Toolchain = &local
	return device, nil
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

// newDeviceRegistrar 创建桌面端本机设备注册与执行循环，各会话的默认文件夹位于用户文档目录下的 Cervi；本机运行时创建失败时不注册设备。
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
	// 无法确定文档目录时默认文件夹放在系统临时目录下，助理照常运行。
	documents, err := devicehost.DocumentsDir()
	if err != nil {
		slog.Warn("无法确定用户文档目录，会话默认文件夹改放在临时目录", "error", err)
		documents = os.TempDir()
	}
	toolchainRoot, toolchainCache, err := toolchain.DefaultDirs()
	if err != nil {
		slog.Error("无法确定 Agent 运行环境目录，本机设备不注册", "error", err)
		return nil
	}
	// 运行环境准备状态变化时通知界面重新读取本机设备，准备结束后立即重新检查待领取运行。
	var worker *devicehost.Worker
	runEnvironment := toolchain.New(toolchainRoot, toolchainCache, func() {
		application.Get().Event.Emit(appservice.LocalDeviceChangedEventName)
		worker.Wake()
	})
	worker = devicehost.NewWorker(registrar, backend, runtime, runEnvironment, filepath.Join(documents, "Cervi"))
	// 本机界面查看本机执行中的运行时直接读取本机过程流。
	backend.UseLocalRunStreams(worker)
	return &desktopDevice{Registrar: registrar, worker: worker, toolchain: runEnvironment}
}
