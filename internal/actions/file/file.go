//go:build server

package file

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const temporaryFileLifetime = 24 * time.Hour

// ErrFileNotFound 表示企业中没有可用的指定文件。
var ErrFileNotFound = errors.New("file not found")

// GetQuery 读取企业文件元数据。
type GetQuery struct {
	db *bun.DB
}

// Location 定义生成公开地址所需的文件位置。
type Location struct {
	ID             string                    `bun:"id"`
	StorageBackend domain.FileStorageBackend `bun:"storage_backend"`
	StorageKey     string                    `bun:"storage_key"`
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
	return get(ctx, q.db, identity.Organization.ID, fileID, "")
}

// ExecuteByStorageKey 返回当前企业中使用指定存储键的文件。
func (q *GetQuery) ExecuteByStorageKey(ctx context.Context, identity *servermodels.Identity, storageKey string) (*servermodels.File, error) {
	if !validIdentity(identity) {
		return nil, common.ErrIdentityInvalid
	}
	record := &servermodels.File{}
	err := q.db.NewSelect().Model(record).
		ColumnExpr("f.*").
		ColumnExpr("(f.expires_at IS NULL OR f.expires_at <= now()) AS expired").
		Where("f.organization_id = ?", identity.Organization.ID).
		Where("f.storage_key = ?", storageKey).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get file by storage key: %w", err)
	}
	return record, nil
}

// ListActiveLocations 批量返回当前企业已关联文件的存储位置。
func (q *GetQuery) ListActiveLocations(ctx context.Context, identity *servermodels.Identity, fileIDs []string) ([]Location, error) {
	if !validIdentity(identity) {
		return nil, common.ErrIdentityInvalid
	}
	fileIDs, valid := common.NormalizeUUIDs(fileIDs)
	if !valid {
		return nil, ErrFileNotFound
	}
	locations := make([]Location, 0, len(fileIDs))
	if len(fileIDs) == 0 {
		return locations, nil
	}
	if err := q.db.NewSelect().TableExpr("files AS f").
		ColumnExpr("f.id::text, f.storage_backend, f.storage_key").
		Where("f.organization_id = ?", identity.Organization.ID).
		Where("f.id IN (?)", bun.In(fileIDs)).
		Where("f.status = ?", domain.FileStatusActive).
		Scan(ctx, &locations); err != nil {
		return nil, fmt.Errorf("list active file locations: %w", err)
	}
	return locations, nil
}

// MarkUploadedAction 将核验通过的文件标记为已上传。
type MarkUploadedAction struct {
	db *bun.DB
}

// NewMarkUploadedAction 创建文件上传完成操作。
func NewMarkUploadedAction(db *bun.DB) *MarkUploadedAction {
	return &MarkUploadedAction{db: db}
}

// Execute 保存文件上传结果并返回最新记录。
func (a *MarkUploadedAction) Execute(ctx context.Context, identity *servermodels.Identity, fileID, etag string) (*servermodels.File, error) {
	if !validIdentity(identity) {
		return nil, common.ErrIdentityInvalid
	}
	record := &servermodels.File{}
	// 过期时间统一使用数据库时钟，与同一语句里 expires_at > now() 的比较保持同源。
	query := a.db.NewUpdate().Model(record).
		Set("status = ?", domain.FileStatusUploaded).
		Set("etag = ?", common.OptionalString(strings.TrimSpace(etag))).
		Set("uploaded_at = now()").
		Set("expires_at = now() + make_interval(secs => ?)", temporaryFileLifetime.Seconds()).
		Set("updated_at = now()").
		Where("f.id = ?", fileID).
		Where("f.organization_id = ?", identity.Organization.ID).
		Where("f.status = ?", domain.FileStatusPending).
		Where("f.expires_at > now()").
		Returning("*")
	result, err := query.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark file uploaded: %w", err)
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
func get(ctx context.Context, db *bun.DB, organizationID, fileID string, status domain.FileStatus) (*servermodels.File, error) {
	if !common.ValidUUID(fileID) {
		return nil, ErrFileNotFound
	}
	record := &servermodels.File{}
	// 过期标记用数据库时钟计算，与写入侧 now() + make_interval(...) 的时钟保持同源。
	query := db.NewSelect().Model(record).
		ColumnExpr("f.*").
		ColumnExpr("(f.expires_at IS NULL OR f.expires_at <= now()) AS expired").
		Where("f.id = ?", fileID)
	if organizationID != "" {
		query = query.Where("f.organization_id = ?", organizationID)
	}
	if status != "" {
		query = query.Where("f.status = ?", status)
	}
	if err := query.Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFileNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	return record, nil
}
