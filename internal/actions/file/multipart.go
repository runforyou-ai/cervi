//go:build server

package file

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// SetMultipartUpload 保存已创建的对象存储分片会话。
func (a *CreateUploadAction) SetMultipartUpload(ctx context.Context, identity *servermodels.Identity, fileID, uploadID string) error {
	return a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		result, err := tx.NewUpdate().Model((*servermodels.File)(nil)).
			Set("multipart_upload_id = ?", uploadID).
			Where("id = ? AND organization_id = ? AND created_by_user_id = ?", fileID, identity.Organization.ID, identity.User.ID).
			Where("status = ? AND expires_at > now()", domain.FileStatusPending).Exec(ctx)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrFileNotFound
		}
		return nil
	})
}

// Cancel 将当前用户尚未发送的文件标记为待清理。
func (a *CreateUploadAction) Cancel(ctx context.Context, identity *servermodels.Identity, fileID string) error {
	return a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		// 已关联消息的文件由附件状态接口管理，不能直接清理。
		_, err := tx.NewUpdate().Model((*servermodels.File)(nil)).
			Set("status = ?", domain.FileStatusDeleting).Set("expires_at = now()").Set("updated_at = now()").
			Where("id = ? AND organization_id = ? AND created_by_user_id = ?", fileID, identity.Organization.ID, identity.User.ID).
			Where("status IN (?, ?)", domain.FileStatusPending, domain.FileStatusUploaded).
			Where("NOT EXISTS (SELECT 1 FROM message_attachments ma WHERE ma.file_id = f.id)").Exec(ctx)
		if err != nil {
			return fmt.Errorf("cancel file upload: %w", err)
		}
		return nil
	})
}

// UploadPartSize 返回有效分片的预期字节数。
func UploadPartSize(record *servermodels.File, number int32) (int64, error) {
	if record.PartSize <= 0 || number <= 0 || int64(number) > (record.ByteSize-1)/record.PartSize+1 {
		return 0, ErrFileNotFound
	}
	return min(record.PartSize, record.ByteSize-int64(number-1)*record.PartSize), nil
}
