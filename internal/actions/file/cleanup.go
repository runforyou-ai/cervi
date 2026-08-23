//go:build server

package file

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/taskruntime"
	"github.com/uptrace/bun"
)

const (
	ScanExpiredActionName   = "file.scan_expired"
	DeleteExpiredActionName = "file.delete_expired"
	CleanupScheduleKey      = "file.cleanup"

	cleanupBatchSize = 200
)

// ScanExpiredInput 定义扫描过期临时文件的输入。
type ScanExpiredInput struct{}

// DeleteExpiredInput 定义删除单个过期临时文件的输入。
type DeleteExpiredInput struct {
	FileID string `json:"file_id"`
}

// ScanExpiredAction 把过期临时文件逐个提交给异步删除 Action。
type ScanExpiredAction struct {
	db       *bun.DB
	enqueuer taskruntime.Enqueuer
}

// NewScanExpiredAction 创建过期文件扫描 Action。
func NewScanExpiredAction(db *bun.DB, enqueuer taskruntime.Enqueuer) *ScanExpiredAction {
	return &ScanExpiredAction{db: db, enqueuer: enqueuer}
}

// Execute 扫描所有过期候选并幂等投递删除任务。
func (a *ScanExpiredAction) Execute(ctx context.Context, _ ScanExpiredInput) error {
	type candidate struct {
		ID        string    `bun:"id"`
		ExpiresAt time.Time `bun:"expires_at"`
	}
	var cursorExpiresAt time.Time
	var cursorID string
	for {
		records := make([]candidate, 0, cleanupBatchSize)
		query := a.db.NewSelect().Table("files").
			Column("id", "expires_at").
			Where("status IN (?, ?, ?)", domain.FileStatusPending, domain.FileStatusUploaded, domain.FileStatusDeleting).
			Where("expires_at <= ?", time.Now().UTC()).
			OrderExpr("expires_at ASC, id ASC").
			Limit(cleanupBatchSize)
		if !cursorExpiresAt.IsZero() {
			query = query.Where("(expires_at, id) > (?, ?)", cursorExpiresAt, cursorID)
		}
		if err := query.Scan(ctx, &records); err != nil {
			return fmt.Errorf("scan expired files: %w", err)
		}
		for _, record := range records {
			if _, err := a.enqueuer.Enqueue(ctx, DeleteExpiredActionName, DeleteExpiredInput{FileID: record.ID}, taskruntime.EnqueueOptions{
				Queue: "files", MaxAttempts: 10,
				IdempotencyKey: "file:" + record.ID,
				TriggerType:    taskruntime.TriggerBusiness,
			}); err != nil {
				return fmt.Errorf("enqueue expired file %s: %w", record.ID, err)
			}
		}
		if len(records) < cleanupBatchSize {
			return nil
		}
		last := records[len(records)-1]
		cursorExpiresAt, cursorID = last.ExpiresAt, last.ID
	}
}

// ContentDeleter 删除文件记录指向的内容。
type ContentDeleter interface {
	Delete(context.Context, *servermodels.File) error
}

// DeleteExpiredAction 删除一个仍处于过期状态的临时文件。
type DeleteExpiredAction struct {
	db      *bun.DB
	deleter ContentDeleter
}

// NewDeleteExpiredAction 创建过期文件删除 Action。
func NewDeleteExpiredAction(db *bun.DB, deleter ContentDeleter) *DeleteExpiredAction {
	return &DeleteExpiredAction{db: db, deleter: deleter}
}

// Execute 重新校验文件状态后删除内容和元数据。
func (a *DeleteExpiredAction) Execute(ctx context.Context, input DeleteExpiredInput) error {
	if input.FileID == "" {
		return taskruntime.Permanent(errors.New("file id is required"))
	}
	var record servermodels.File
	err := a.db.NewRaw(`
		UPDATE files
		SET status = ?, updated_at = now()
		WHERE id = ?
			AND expires_at <= now()
			AND status IN (?, ?, ?)
		RETURNING *
	`, domain.FileStatusDeleting, input.FileID,
		domain.FileStatusPending, domain.FileStatusUploaded, domain.FileStatusDeleting).Scan(ctx, &record)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("claim expired file: %w", err)
	}
	if err := a.deleter.Delete(ctx, &record); err != nil {
		return err
	}
	if _, err := a.db.NewDelete().Model((*servermodels.File)(nil)).
		Where("id = ?", record.ID).
		Where("status = ?", domain.FileStatusDeleting).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete expired file record: %w", err)
	}
	slog.Info("过期文件已清理", "organization_id", record.OrganizationID, "file_id", record.ID, "storage_backend", record.StorageBackend)
	return nil
}
