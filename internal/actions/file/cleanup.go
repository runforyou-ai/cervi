//go:build server

package file

import (
	"context"
	"fmt"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CleanupAction 安排并删除过期文件记录。
type CleanupAction struct {
	db *bun.DB
}

// NewCleanupAction 创建过期文件清理操作。
func NewCleanupAction(db *bun.DB) *CleanupAction {
	return &CleanupAction{db: db}
}

// ScheduleExpired 分批将过期临时文件放入删除队列。
func (a *CleanupAction) ScheduleExpired(ctx context.Context, now, deleteAt time.Time, limit int) (int64, error) {
	result, err := a.db.NewRaw(`
		WITH candidates AS (
			SELECT id
			FROM files
			WHERE status IN (?, ?)
				AND expires_at <= ?
			ORDER BY expires_at ASC, id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT ?
		)
		UPDATE files AS f
		SET status = ?, expires_at = ?, updated_at = now()
		FROM candidates AS c
		WHERE f.id = c.id
	`, domain.FileStatusPending, domain.FileStatusUploaded, now, limit, domain.FileStatusDeleting, deleteAt).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("schedule expired files: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read scheduled file count: %w", err)
	}
	return count, nil
}

// ClaimDeleting 分批认领到期的删除任务。
func (a *CleanupAction) ClaimDeleting(ctx context.Context, now, retryAt time.Time, limit int) ([]servermodels.File, error) {
	records := make([]servermodels.File, 0)
	err := a.db.NewRaw(`
		WITH candidates AS (
			SELECT id
			FROM files
			WHERE status = ?
				AND expires_at <= ?
			ORDER BY expires_at ASC, id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT ?
		)
		UPDATE files AS f
		SET status = ?, expires_at = ?, updated_at = now()
		FROM candidates AS c
		WHERE f.id = c.id
		RETURNING f.*
	`, domain.FileStatusDeleting, now, limit, domain.FileStatusDeleting, retryAt).Scan(ctx, &records)
	if err != nil {
		return nil, fmt.Errorf("claim deleting files: %w", err)
	}
	return records, nil
}

// DeleteClaimed 删除已清理内容的文件记录。
func (a *CleanupAction) DeleteClaimed(ctx context.Context, fileID string) error {
	if _, err := a.db.NewDelete().
		Model((*servermodels.File)(nil)).
		Where("id = ?", fileID).
		Where("status = ?", domain.FileStatusDeleting).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete claimed file: %w", err)
	}
	return nil
}
