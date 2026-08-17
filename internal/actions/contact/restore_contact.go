//go:build server

package contact

import (
	"context"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// RestoreContactAction 从回收站恢复联系人。
type RestoreContactAction struct {
	db *bun.DB
}

// NewRestoreContactAction 创建联系人恢复操作。
func NewRestoreContactAction(db *bun.DB) *RestoreContactAction {
	return &RestoreContactAction{db: db}
}

// Execute 恢复当前企业中已软删除的联系人。
func (a *RestoreContactAction) Execute(ctx context.Context, principal *servermodels.Principal, contactID string) (*ContactDetail, error) {
	if !validUUID(contactID) {
		return nil, ErrNotFound
	}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validatePrincipal(ctx, tx, principal); err != nil {
			return err
		}
		result, err := tx.NewUpdate().
			Table("contacts").
			Set("deleted_at = NULL").
			Set("updated_at = now()").
			Where("id = ?", contactID).
			Where("organization_id = ?", principal.Organization.ID).
			Where("deleted_at IS NOT NULL").
			Exec(ctx)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("restore contact: %w", err)
	}
	return NewGetContactQuery(a.db).Execute(ctx, principal, contactID)
}
