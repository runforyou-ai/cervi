//go:build server

package file

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ErrFileNotFound 表示企业中没有可用的指定文件。
var ErrFileNotFound = errors.New("file not found")

// GetQuery 读取企业文件元数据。
type GetQuery struct {
	db *bun.DB
}

// NewGetQuery 创建文件查询。
func NewGetQuery(db *bun.DB) *GetQuery {
	return &GetQuery{db: db}
}

// Execute 返回当前企业中的指定文件。
func (q *GetQuery) Execute(ctx context.Context, identity *servermodels.Identity, fileID string) (*servermodels.File, error) {
	if !validIdentity(identity) {
		return nil, common.ErrIdentityInvalid
	}
	return get(ctx, q.db, identity.Organization.ID, fileID, false)
}

// GetReadyByID 返回已上传且未删除的指定文件。
func (q *GetQuery) GetReadyByID(ctx context.Context, fileID string) (*servermodels.File, error) {
	return get(ctx, q.db, "", fileID, true)
}

// MarkReadyAction 将核验通过的文件标记为已上传。
type MarkReadyAction struct {
	db *bun.DB
}

// NewMarkReadyAction 创建文件完成操作。
func NewMarkReadyAction(db *bun.DB) *MarkReadyAction {
	return &MarkReadyAction{db: db}
}

// Execute 保存文件上传结果并返回最新记录。
func (a *MarkReadyAction) Execute(ctx context.Context, identity *servermodels.Identity, fileID, etag string) (*servermodels.File, error) {
	if !validIdentity(identity) {
		return nil, common.ErrIdentityInvalid
	}
	record := &servermodels.File{}
	query := a.db.NewUpdate().Model(record).
		Set("status = ?", domain.FileStatusReady).
		Set("etag = ?", optionalString(etag)).
		Set("uploaded_at = COALESCE(uploaded_at, now())").
		Set("updated_at = now()").
		Where("f.id = ?", fileID).
		Where("f.organization_id = ?", identity.Organization.ID).
		Where("f.deleted_at IS NULL").
		Returning("*")
	result, err := query.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark file ready: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read marked file count: %w", err)
	}
	if rows == 0 {
		return nil, ErrFileNotFound
	}
	return record, nil
}

// get 按文件和可选企业范围读取元数据。
func get(ctx context.Context, db *bun.DB, organizationID, fileID string, ready bool) (*servermodels.File, error) {
	if !common.ValidUUID(fileID) {
		return nil, ErrFileNotFound
	}
	record := &servermodels.File{}
	query := db.NewSelect().Model(record).Where("f.id = ?", fileID).Where("f.deleted_at IS NULL")
	if organizationID != "" {
		query = query.Where("f.organization_id = ?", organizationID)
	}
	if ready {
		query = query.Where("f.status = ?", domain.FileStatusReady)
	}
	if err := query.Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFileNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	return record, nil
}

// optionalString 把空字符串转换为空值。
func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
