//go:build !server && !ios && !android

package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/runforyou-ai/cervi/internal/apiproxy"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/runforyou-ai/cervi/internal/devicehost"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

// nativeStorage 组合桌面端连接、登录凭据与设备注册存储能力。
type nativeStorage interface {
	apiproxy.Store
	clientsession.Store
	devicehost.Store
}

// desktopDevice 组合桌面端本机设备注册与 Agent 运行执行循环。
type desktopDevice struct {
	*devicehost.Registrar
	worker *devicehost.Worker
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

// newDeviceRegistrar 创建桌面端本机设备注册与执行循环，各会话的默认文件夹位于用户文稿目录下的 Cervi；本机运行时创建失败或无法确定用户主目录时不注册设备。
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
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("无法确定用户主目录，本机设备不注册", "error", err)
		return nil
	}
	worker := devicehost.NewWorker(registrar, backend, runtime, filepath.Join(home, "Documents", "Cervi"))
	// 本机界面查看本机执行中的运行时直接读取本机过程流。
	backend.UseLocalRunStreams(worker)
	return &desktopDevice{Registrar: registrar, worker: worker}
}
