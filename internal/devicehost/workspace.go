//go:build !server && !ios && !android

package devicehost

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// WorkspaceClient 是注册本机工作区使用的企业服务端调用。
type WorkspaceClient interface {
	// RegisterDeviceWorkspace 在当前用户的设备上注册工作区。
	RegisterDeviceWorkspace(context.Context, appservice.RequestMeta, string, appservice.DeviceWorkspaceInput) (appservice.DeviceWorkspace, error)
	// ListDeviceWorkspaces 返回当前用户设备上的工作区。
	ListDeviceWorkspaces(context.Context, appservice.RequestMeta, string) (appservice.DeviceWorkspaceList, error)
}

// LocalWorkspaceStore 保存本机工作区的本地路径。
type LocalWorkspaceStore interface {
	// SaveAgentWorkspace 保存本机工作区在指定企业服务器上的编号与本地路径。
	SaveAgentWorkspace(ctx context.Context, serverURL, organizationID, workspaceID, path string) error
	// AgentWorkspaceIDsByPath 返回指定企业服务器上本地路径相同的工作区编号。
	AgentWorkspaceIDsByPath(ctx context.Context, serverURL, organizationID, path string) ([]string, error)
}

// DirectorySelector 让用户选择本机目录，用户取消时返回空路径。
type DirectorySelector func(context.Context, appservice.RequestMeta) (string, error)

// Workspaces 把用户选择的本机目录注册为本设备的工作区。
type Workspaces struct {
	registrar *Registrar
	store     LocalWorkspaceStore
	client    WorkspaceClient
	selector  DirectorySelector
}

// NewWorkspaces 创建本机工作区管理；当前平台不注册本机设备时返回 nil。
func NewWorkspaces(registrar *Registrar, store LocalWorkspaceStore, client WorkspaceClient, selector DirectorySelector) *Workspaces {
	if registrar == nil {
		return nil
	}
	return &Workspaces{registrar: registrar, store: store, client: client, selector: selector}
}

// AddLocalWorkspace 让用户选择本机目录，已注册过的目录直接返回原工作区，否则以目录名为显示名注册到本设备并在本地保存真实路径；用户取消选择时返回空工作区编号。
func (w *Workspaces) AddLocalWorkspace(ctx context.Context, meta appservice.RequestMeta) (appservice.DeviceWorkspace, error) {
	session, found, err := w.registrar.currentDeviceSession(ctx, meta)
	if err != nil {
		slog.Warn("读取本机设备注册状态失败", "error", err)
		return appservice.DeviceWorkspace{}, appservice.FailedError(meta, cervii18n.ErrorDeviceWorkspaceRegisterFailed)
	}
	if !found {
		return appservice.DeviceWorkspace{}, appservice.FailedError(meta, cervii18n.ErrorDeviceWorkspaceRegisterFailed)
	}
	path, err := w.selector(ctx, meta)
	if err != nil {
		return appservice.DeviceWorkspace{}, appservice.FailedError(meta, cervii18n.ErrorDeviceWorkspaceRegisterFailed)
	}
	if path == "" {
		return appservice.DeviceWorkspace{}, nil
	}
	// 按解析符号链接后的真实目录识别工作区，同一目录复用已注册的工作区。
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	path = filepath.Clean(path)
	existing, found, err := w.existingWorkspace(ctx, meta, session, path)
	if err != nil {
		slog.Warn("读取本机已有工作区失败", "device_id", session.deviceID, "error", err)
		return appservice.DeviceWorkspace{}, err
	}
	if found {
		return existing, nil
	}
	workspace, err := w.client.RegisterDeviceWorkspace(ctx, meta, session.deviceID, appservice.DeviceWorkspaceInput{Label: filepath.Base(path)})
	if err != nil {
		return appservice.DeviceWorkspace{}, err
	}
	if err := w.store.SaveAgentWorkspace(ctx, session.serverURL, session.credential.OrganizationID, workspace.ID, path); err != nil {
		slog.Warn("保存本机工作区路径失败", "workspace_id", workspace.ID, "error", err)
		return appservice.DeviceWorkspace{}, appservice.FailedError(meta, cervii18n.ErrorDeviceWorkspaceRegisterFailed)
	}
	slog.Info("本机工作区已添加", "device_id", session.deviceID, "workspace_id", workspace.ID)
	return workspace, nil
}

// existingWorkspace 返回本设备上已注册且本地路径相同的工作区。
func (w *Workspaces) existingWorkspace(ctx context.Context, meta appservice.RequestMeta, session deviceSession, path string) (appservice.DeviceWorkspace, bool, error) {
	ids, err := w.store.AgentWorkspaceIDsByPath(ctx, session.serverURL, session.credential.OrganizationID, path)
	if err != nil || len(ids) == 0 {
		return appservice.DeviceWorkspace{}, false, err
	}
	list, err := w.client.ListDeviceWorkspaces(ctx, meta, session.deviceID)
	if err != nil {
		return appservice.DeviceWorkspace{}, false, err
	}
	for _, workspace := range list.Workspaces {
		for _, id := range ids {
			if workspace.ID == id {
				return workspace, true, nil
			}
		}
	}
	return appservice.DeviceWorkspace{}, false, nil
}
