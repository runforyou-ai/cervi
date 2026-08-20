//go:build server

package contact

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateContactAction 修改外部联系人。
type UpdateContactAction struct {
	db *bun.DB
}

// NewUpdateContactAction 创建联系人修改操作。
func NewUpdateContactAction(db *bun.DB) *UpdateContactAction {
	return &UpdateContactAction{db: db}
}

// Execute 校验并更新当前企业的联系人及联系方式。
func (a *UpdateContactAction) Execute(ctx context.Context, identity *servermodels.Identity, contactID string, input ContactInput) (*ContactDetail, error) {
	if !common.ValidUUID(contactID) {
		return nil, ErrNotFound
	}
	input, fields := normalizeContactInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}

	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		var sourceChannelID string
		if err := tx.NewSelect().
			Table("contacts").
			Column("source_channel_id").
			Where("id = ?", contactID).
			Where("organization_id = ?", identity.Organization.ID).
			Where("deleted_at IS NULL").
			For("UPDATE").
			Scan(ctx, &sourceChannelID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if sourceChannelID != input.ChannelID {
			return &ValidationError{Fields: map[string]ValidationCode{"channelId": ValidationChannelImmutable}}
		}
		query := tx.NewUpdate().
			Table("contacts").
			Set("stage = ?", input.Stage).
			Set("updated_at = now()")
		if input.DisplayName != "" {
			query = query.Set("display_name = ?", input.DisplayName)
		} else {
			query = query.Set("display_name = NULL")
		}
		if input.Notes != "" {
			query = query.Set("notes = ?", input.Notes)
		} else {
			query = query.Set("notes = NULL")
		}
		_, err := query.
			Where("id = ?", contactID).
			Where("organization_id = ?", identity.Organization.ID).
			Where("deleted_at IS NULL").
			Exec(ctx)
		if err != nil {
			return err
		}
		return replaceMethods(ctx, tx, identity.Organization.ID, contactID, input.Methods)
	})
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}
	return NewGetContactQuery(a.db).Execute(ctx, identity, contactID)
}
