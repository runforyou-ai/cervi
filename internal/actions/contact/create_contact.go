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
	input, fields := normalizeContactInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var contact *servermodels.Contact
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validatePrincipal(ctx, tx, principal); err != nil {
			return err
		}
		if err := validateSourceChannel(ctx, tx, principal.Organization.ID, input.ChannelID); err != nil {
			return err
		}
		contact = &servermodels.Contact{
			OrganizationID:  principal.Organization.ID,
			CreatedByUserID: principal.User.ID,
			SourceChannelID: input.ChannelID,
			Stage:           string(input.Stage),
		}
		if input.DisplayName != "" {
			contact.DisplayName = &input.DisplayName
		}
		if input.Notes != "" {
			contact.Notes = &input.Notes
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
