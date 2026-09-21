//go:build server

package device

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const (
	ValidationWorkspaceLabelRequired ValidationCode = "DEVICE_WORKSPACE_LABEL_REQUIRED"
	ValidationWorkspaceLabelTooLong  ValidationCode = "DEVICE_WORKSPACE_LABEL_TOO_LONG"
)

// maxWorkspaceLabelLength 是工作区显示名的最大字符数。
const maxWorkspaceLabelLength = 100

// ErrWorkspaceNotFound 表示当前用户的未撤销设备上不存在指定工作区。
var ErrWorkspaceNotFound = errors.New("device workspace not found")

// WorkspaceRecord 定义设备工作区记录。
type WorkspaceRecord struct {
	ID         string
	DeviceID   string
	Label      string
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

// RegisterWorkspaceAction 在当前用户的设备上注册工作区。
type RegisterWorkspaceAction struct {
	db *bun.DB
}

// NewRegisterWorkspaceAction 创建工作区注册操作。
func NewRegisterWorkspaceAction(db *bun.DB) *RegisterWorkspaceAction {
	return &RegisterWorkspaceAction{db: db}
}

// Execute 在当前用户未撤销的设备上注册工作区，只保存显示名，真实路径由设备本地保存。
func (a *RegisterWorkspaceAction) Execute(ctx context.Context, identity *servermodels.Identity, deviceID, label string) (*WorkspaceRecord, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"label": ValidationWorkspaceLabelRequired}}
	}
	if utf8.RuneCountInString(label) > maxWorkspaceLabelLength {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"label": ValidationWorkspaceLabelTooLong}}
	}
	workspace := servermodels.DeviceWorkspace{OrganizationID: identity.Organization.ID, DeviceID: deviceID, Label: label}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if err := lockOwnDevice(ctx, tx, identity, deviceID); err != nil {
			return err
		}
		_, err := tx.NewInsert().Model(&workspace).
			Column("organization_id", "device_id", "label").
			Returning("*").Exec(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("register device workspace: %w", err)
	}
	slog.Info("设备工作区已注册", "organization_id", identity.Organization.ID, "device_id", deviceID, "workspace_id", workspace.ID)
	output := workspaceFromModel(workspace)
	return &output, nil
}

// ListWorkspacesQuery 查询当前用户设备上的工作区。
type ListWorkspacesQuery struct {
	db *bun.DB
}

// NewListWorkspacesQuery 创建工作区列表查询。
func NewListWorkspacesQuery(db *bun.DB) *ListWorkspacesQuery {
	return &ListWorkspacesQuery{db: db}
}

// Execute 返回当前用户指定设备上的工作区，按最近使用和注册时间排列。
func (q *ListWorkspacesQuery) Execute(ctx context.Context, identity *servermodels.Identity, deviceID string) ([]WorkspaceRecord, error) {
	workspaces := make([]servermodels.DeviceWorkspace, 0)
	if err := q.db.NewSelect().Model(&workspaces).
		Join("JOIN devices AS d ON d.id = dw.device_id AND d.organization_id = dw.organization_id").
		Where("dw.organization_id = ? AND dw.device_id = ?", identity.Organization.ID, deviceID).
		Where("d.user_id = ? AND d.revoked_at IS NULL", identity.User.ID).
		OrderExpr("dw.last_used_at DESC NULLS LAST, dw.created_at DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list device workspaces: %w", err)
	}
	output := make([]WorkspaceRecord, 0, len(workspaces))
	for _, workspace := range workspaces {
		output = append(output, workspaceFromModel(workspace))
	}
	return output, nil
}

// lockOwnDevice 在调用方事务中锁定当前用户未撤销的设备。
func lockOwnDevice(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, deviceID string) error {
	var id string
	err := tx.NewSelect().Model((*servermodels.Device)(nil)).Column("id").
		Where("d.organization_id = ? AND d.user_id = ? AND d.id = ?", identity.Organization.ID, identity.User.ID, deviceID).
		Where("d.revoked_at IS NULL").
		For("SHARE").Scan(ctx, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// workspaceFromModel 转换工作区存储模型。
func workspaceFromModel(input servermodels.DeviceWorkspace) WorkspaceRecord {
	return WorkspaceRecord{ID: input.ID, DeviceID: input.DeviceID, Label: input.Label, LastUsedAt: input.LastUsedAt, CreatedAt: input.CreatedAt}
}
