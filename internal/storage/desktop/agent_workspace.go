//go:build !server && !ios && !android

package desktop

import (
	"context"
	"database/sql"
	"errors"
	"time"

	desktopmodels "github.com/runforyou-ai/cervi/internal/storage/desktop/models"
)

// SaveAgentWorkspace 保存本机工作区在指定企业服务器上的编号与本地路径。
func (s *Store) SaveAgentWorkspace(ctx context.Context, serverURL, organizationID, workspaceID, path string) error {
	workspace := &desktopmodels.AgentWorkspace{
		WorkspaceID:    workspaceID,
		ServerURL:      serverURL,
		OrganizationID: organizationID,
		Path:           path,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	_, err := s.db.NewInsert().
		Model(workspace).
		Column("workspace_id", "server_url", "organization_id", "path", "created_at").
		On("CONFLICT (workspace_id) DO UPDATE").
		Set("path = EXCLUDED.path").
		Exec(ctx)
	return err
}

// LoadAgentWorkspacePath 读取指定企业服务器上工作区编号对应的本地路径。
func (s *Store) LoadAgentWorkspacePath(ctx context.Context, serverURL, organizationID, workspaceID string) (string, bool, error) {
	workspace := &desktopmodels.AgentWorkspace{}
	err := s.db.NewSelect().
		Model(workspace).
		Where("workspace_id = ?", workspaceID).
		Where("server_url = ?", serverURL).
		Where("organization_id = ?", organizationID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return workspace.Path, true, nil
}

// AgentWorkspaceIDsByPath 返回指定企业服务器上本地路径相同的工作区编号，按保存时间排列。
func (s *Store) AgentWorkspaceIDsByPath(ctx context.Context, serverURL, organizationID, path string) ([]string, error) {
	ids := make([]string, 0)
	err := s.db.NewSelect().
		Model((*desktopmodels.AgentWorkspace)(nil)).
		Column("workspace_id").
		Where("server_url = ?", serverURL).
		Where("organization_id = ?", organizationID).
		Where("path = ?", path).
		Order("created_at").
		Scan(ctx, &ids)
	return ids, err
}
