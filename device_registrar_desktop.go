//go:build !server && !ios && !android

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/runforyou-ai/cervi/internal/apiproxy"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/runforyou-ai/cervi/internal/devicehost"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/localmcp"
	"github.com/runforyou-ai/cervi/internal/integration/toolchain"
	"github.com/runforyou-ai/cervi/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// localMCPConfigName 是数据目录中本地 MCP 配置文件的名称。
const localMCPConfigName = "mcp.json"

// nativeStorage 组合桌面端连接、登录凭据与设备注册存储能力。
type nativeStorage interface {
	apiproxy.Store
	clientsession.Store
	devicehost.Store
}

// desktopDevice 组合桌面端本机设备注册、Agent 运行执行循环、运行环境与本地 MCP 服务配置。
type desktopDevice struct {
	*devicehost.Registrar
	worker    *devicehost.Worker
	toolchain *toolchain.Manager
	localMCP  *localmcp.Store
}

// CurrentDevice 返回本机设备注册状态与 Agent 运行环境的准备状态。
func (d *desktopDevice) CurrentDevice(ctx context.Context, meta appservice.RequestMeta) (appservice.LocalDevice, error) {
	device, err := d.Registrar.CurrentDevice(ctx, meta)
	if err != nil {
		return device, err
	}
	local := d.toolchainStatus()
	device.Toolchain = &local
	return device, nil
}

// LocalEnvironment 返回运行环境的状态、安装位置、各组件版本与本地 MCP 服务。
func (d *desktopDevice) LocalEnvironment(context.Context, appservice.RequestMeta) (appservice.LocalEnvironment, error) {
	info := d.toolchain.Info()
	servers, err := d.localMCP.List()
	if err != nil {
		return appservice.LocalEnvironment{}, err
	}
	environment := appservice.LocalEnvironment{
		Toolchain: d.toolchainStatus(), Location: info.Root, UVVersion: info.UV, NodeVersion: info.Node, PythonVersion: info.Python,
		MCPServers: make([]appservice.LocalMCPServer, 0, len(servers)),
	}
	for _, server := range servers {
		environment.MCPServers = append(environment.MCPServers, appservice.LocalMCPServer{
			Name: server.Name, Type: appservice.LocalMCPServerType(server.Transport()), Command: server.Command, Args: server.Args, URL: server.URL,
		})
	}
	return environment, nil
}

// UpdateLocalToolchain 把运行环境更新到下载源的最新版本，失败原因转换为本地化错误。
func (d *desktopDevice) UpdateLocalToolchain(ctx context.Context, meta appservice.RequestMeta) (appservice.LocalToolchainUpdate, error) {
	updated, err := d.toolchain.Update(ctx)
	switch {
	case err == nil:
		return appservice.LocalToolchainUpdate{Updated: updated}, nil
	case errors.Is(err, toolchain.ErrBusy):
		return appservice.LocalToolchainUpdate{}, appservice.ConflictError(meta, cervii18n.ErrorLocalToolchainBusy, "toolchain_busy")
	case errors.Is(err, toolchain.ErrNotReady):
		return appservice.LocalToolchainUpdate{}, appservice.ConflictError(meta, cervii18n.ErrorLocalToolchainNotReady, "toolchain_not_ready")
	}
	slog.Warn("更新 Agent 运行环境失败", "error", err)
	key := map[toolchain.Failure]cervii18n.Key{
		toolchain.FailureDownload: cervii18n.ErrorLocalToolchainDownload,
		toolchain.FailureVerify:   cervii18n.ErrorLocalToolchainVerify,
		toolchain.FailureInstall:  cervii18n.ErrorLocalToolchainInstall,
	}[toolchain.FailureOf(err)]
	return appservice.LocalToolchainUpdate{}, appservice.FailedError(meta, key)
}

// UninstallLocalToolchain 删除运行环境的全部文件与下载缓存，重新安装前不再自动安装。
func (d *desktopDevice) UninstallLocalToolchain(_ context.Context, meta appservice.RequestMeta) error {
	err := d.toolchain.Uninstall()
	switch {
	case err == nil:
		return nil
	case errors.Is(err, toolchain.ErrBusy):
		return appservice.ConflictError(meta, cervii18n.ErrorLocalToolchainBusy, "toolchain_busy")
	}
	slog.Warn("卸载 Agent 运行环境失败", "error", err)
	return appservice.FailedError(meta, cervii18n.ErrorLocalToolchainUninstall)
}

// InstallLocalToolchain 清除卸载记录并在后台重新安装运行环境。
func (d *desktopDevice) InstallLocalToolchain(context.Context, appservice.RequestMeta) error {
	if err := d.toolchain.Install(); err != nil {
		return err
	}
	// 重新安装期间设备不领取运行，安装完成后经 Wake 重新检查。
	d.worker.Wake()
	return nil
}

// OpenLocalToolchainFolder 在系统文件管理器中打开运行环境的安装位置。
func (d *desktopDevice) OpenLocalToolchainFolder(context.Context, appservice.RequestMeta) error {
	root := d.toolchain.Info().Root
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	return application.Get().Env.OpenFileManager(root, false)
}

// RemoveLocalMCPServer 删除本地 MCP 服务配置。
func (d *desktopDevice) RemoveLocalMCPServer(_ context.Context, meta appservice.RequestMeta, name string) error {
	removed, err := d.localMCP.Remove(name)
	if err != nil {
		return err
	}
	if !removed {
		return appservice.NotFoundError(meta, cervii18n.ErrorLocalMCPServerNotFound)
	}
	return nil
}

// toolchainStatus 返回运行环境的准备状态。
func (d *desktopDevice) toolchainStatus() appservice.LocalToolchain {
	status := d.toolchain.Status()
	local := appservice.LocalToolchain{State: appservice.LocalToolchainState(status.State), Updating: status.Updating}
	if status.Failure != "" {
		failure := appservice.LocalToolchainFailure(status.Failure)
		local.Failure = &failure
	}
	return local
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
	dataDirectory, err := storage.DesktopDataDirectory()
	if err != nil {
		slog.Error("无法确定桌面端数据目录，本机设备不注册", "error", err)
		return nil
	}
	// 本机环境变化时通知界面重新读取本机设备与本机环境。
	notify := func() { application.Get().Event.Emit(appservice.LocalDeviceChangedEventName) }
	// 运行环境准备结束后立即重新检查待领取运行。
	var worker *devicehost.Worker
	runEnvironment := toolchain.New(toolchainRoot, toolchainCache, func() {
		notify()
		worker.Wake()
	})
	localMCP := localmcp.NewStore(filepath.Join(dataDirectory, localMCPConfigName), notify)
	worker = devicehost.NewWorker(registrar, backend, runtime, runEnvironment, localMCP, filepath.Join(documents, "Cervi"))
	// 本机界面查看本机执行中的运行时直接读取本机过程流。
	backend.UseLocalRunStreams(worker)
	return &desktopDevice{Registrar: registrar, worker: worker, toolchain: runEnvironment, localMCP: localMCP}
}
