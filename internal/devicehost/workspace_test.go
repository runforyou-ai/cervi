//go:build !server && !ios && !android

package devicehost

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/runforyou-ai/cervi/internal/appservice"
)

// stubWorkspaceClient 在内存中注册设备工作区。
type stubWorkspaceClient struct {
	workspaces []appservice.DeviceWorkspace
}

// RegisterDeviceWorkspace 按注册顺序生成工作区编号。
func (c *stubWorkspaceClient) RegisterDeviceWorkspace(_ context.Context, _ appservice.RequestMeta, deviceID string, input appservice.DeviceWorkspaceInput) (appservice.DeviceWorkspace, error) {
	workspace := appservice.DeviceWorkspace{ID: "workspace-" + string(rune('1'+len(c.workspaces))), DeviceID: deviceID, Label: input.Label}
	c.workspaces = append(c.workspaces, workspace)
	return workspace, nil
}

// ListDeviceWorkspaces 返回已注册的工作区。
func (c *stubWorkspaceClient) ListDeviceWorkspaces(context.Context, appservice.RequestMeta, string) (appservice.DeviceWorkspaceList, error) {
	return appservice.DeviceWorkspaceList{Workspaces: c.workspaces}, nil
}

// stubLocalWorkspaces 在内存中保存工作区路径。
type stubLocalWorkspaces map[string]string

// SaveAgentWorkspace 保存工作区路径。
func (s stubLocalWorkspaces) SaveAgentWorkspace(_ context.Context, _, _, workspaceID, path string) error {
	s[workspaceID] = path
	return nil
}

// AgentWorkspaceIDsByPath 返回路径相同的工作区编号。
func (s stubLocalWorkspaces) AgentWorkspaceIDsByPath(_ context.Context, _, _, path string) ([]string, error) {
	ids := make([]string, 0)
	for id, saved := range s {
		if saved == path {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// TestAddLocalWorkspaceReusesDirectory 验证经符号链接或重复选择同一目录时复用已注册的工作区。
func TestAddLocalWorkspaceReusesDirectory(t *testing.T) {
	const serverURL = "https://cervi.example.com"
	store := &stubStore{installID: "install-1", registrations: map[string]string{serverURL + "|org-1|user-1": "device-1"}}
	registrar, sessions := newTestRegistrar(t, store, &stubClient{serverURL: serverURL, deviceID: "device-1"})
	if err := sessions.Establish(context.Background(), credentialFor(serverURL, "org-1", "user-1", "token-1")); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(directory, link); err != nil {
		t.Fatal(err)
	}
	selections := []string{directory, link, directory}
	client := &stubWorkspaceClient{}
	workspaces := NewWorkspaces(registrar, stubLocalWorkspaces{}, client, func(context.Context, appservice.RequestMeta) (string, error) {
		selected := selections[0]
		selections = selections[1:]
		return selected, nil
	})

	ids := map[string]bool{}
	for range 3 {
		workspace, err := workspaces.AddLocalWorkspace(context.Background(), appservice.RequestMeta{})
		if err != nil {
			t.Fatal(err)
		}
		ids[workspace.ID] = true
	}
	if len(client.workspaces) != 1 || len(ids) != 1 || client.workspaces[0].Label != "project" {
		t.Fatalf("注册的工作区 = %+v，返回的编号 = %v", client.workspaces, ids)
	}
}
