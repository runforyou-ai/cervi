//go:build server

package contact

import (
	"context"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateContactAction 创建外部联系人。
type CreateContactAction struct {
	db *bun.DB
}

// NewCreateContactAction 创建联系人新增操作。
func NewCreateContactAction(db *bun.DB) *CreateContactAction {
	return &CreateContactAction{db: db}
}

// Execute 校验字段并在当前企业中创建联系人。
func (a *CreateContactAction) Execute(ctx context.Context, principal *servermodels.Principal, input ContactInput) (*ContactDetail, error) {
	input, fields := normalizeContactInput(input, true)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if principal == nil || !validUUID(principal.Organization.ID) || !validUUID(principal.User.ID) || principal.User.OrganizationID != principal.Organization.ID {
		return nil, ErrPrincipalInvalid
	}

	sourceChannelID := input.ChannelID
	contact := &servermodels.Contact{
		OrganizationID:  principal.Organization.ID,
		CreatedByUserID: principal.User.ID,
		SourceChannelID: &sourceChannelID,
		Stage:           input.Stage,
	}
	if input.DisplayName != "" {
		contact.DisplayName = &input.DisplayName
	}
	if input.Notes != "" {
		contact.Notes = &input.Notes
	}

	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validatePrincipal(ctx, tx, principal); err != nil {
			return err
		}
		if err := validateSourceChannel(ctx, tx, principal.Organization.ID, input.ChannelID); err != nil {
			return err
		}
		if _, err := tx.NewInsert().
			Model(contact).
			Column("organization_id", "created_by_user_id", "source_channel_id", "display_name", "stage", "notes").
			Returning("*").
			Exec(ctx); err != nil {
			return err
		}
		return replaceMethods(ctx, tx, contact.OrganizationID, contact.ID, input.Methods)
	})
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}
	return NewGetContactQuery(a.db).Execute(ctx, principal, contact.ID)
}
