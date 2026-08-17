//go:build server

package contact

import (
	"context"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DeleteContactAction 将联系人移入回收站。
type DeleteContactAction struct {
	db *bun.DB
}

// NewDeleteContactAction 创建联系人删除操作。
func NewDeleteContactAction(db *bun.DB) *DeleteContactAction {
	return &DeleteContactAction{db: db}
}

// Execute 软删除当前企业的联系人。
func (a *DeleteContactAction) Execute(ctx context.Context, principal *servermodels.Principal, contactID string) error {
	if !validUUID(contactID) {
		return ErrNotFound
	}
	result, err := a.db.NewUpdate().
		Table("contacts").
		Set("deleted_at = now()").
		Set("updated_at = now()").
		Where("id = ?", contactID).
		Where("organization_id = ?", principal.Organization.ID).
		Where("deleted_at IS NULL").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted contact count: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
