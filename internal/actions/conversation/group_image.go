//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// activateGroupImage 锁定并激活一个已上传的群聊图片。
func activateGroupImage(ctx context.Context, tx bun.Tx, organizationID, fileID string, currentFileID *string) (*string, error) {
	if !common.ValidUUID(fileID) {
		return nil, ErrGroupImageFileNotFound
	}
	file := &servermodels.File{}
	err := tx.NewSelect().Model(file).
		Column("id", "status").
		ColumnExpr("(f.expires_at IS NULL OR f.expires_at <= now()) AS expired").
		Where("f.id = ?", fileID).
		Where("f.organization_id = ?", organizationID).
		Where("f.purpose = ?", domain.FilePurposeGroupImage).
		For("UPDATE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGroupImageFileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock group image: %w", err)
	}
	sameImage := currentFileID != nil && *currentFileID == file.ID
	if file.Status == string(domain.FileStatusUploaded) {
		if file.Expired {
			return nil, ErrGroupImageFileNotFound
		}
		result, err := tx.NewUpdate().Model((*servermodels.File)(nil)).
			Set("status = ?", domain.FileStatusActive).
			Set("expires_at = NULL").
			Set("updated_at = now()").
			Where("id = ?", file.ID).
			Where("status = ?", domain.FileStatusUploaded).
			Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("activate group image: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("read activated group image count: %w", err)
		}
		if rows != 1 {
			return nil, ErrGroupImageFileNotFound
		}
	} else if file.Status != string(domain.FileStatusActive) || !sameImage {
		return nil, ErrGroupImageFileNotFound
	}
	return &file.ID, nil
}

// retireGroupImage 将已经解除群聊关联的旧图片交给清理任务删除。
func retireGroupImage(ctx context.Context, tx bun.Tx, organizationID string, previousFileID, nextFileID *string) error {
	if previousFileID == nil || nextFileID == nil || *previousFileID == *nextFileID {
		return nil
	}
	if _, err := tx.NewUpdate().Model((*servermodels.File)(nil)).
		Set("status = ?", domain.FileStatusDeleting).
		Set("expires_at = now()").
		Set("updated_at = now()").
		Where("id = ?", *previousFileID).
		Where("organization_id = ?", organizationID).
		Where("status = ?", domain.FileStatusActive).
		Exec(ctx); err != nil {
		return fmt.Errorf("retire previous group image: %w", err)
	}
	return nil
}
