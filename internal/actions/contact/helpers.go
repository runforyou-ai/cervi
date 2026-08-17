//go:build server

package contact

import (
	"context"
	"database/sql"
	"errors"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

func validatePrincipal(ctx context.Context, tx bun.Tx, principal *servermodels.Principal) error {
	if principal == nil || !validUUID(principal.Organization.ID) || !validUUID(principal.User.ID) || principal.User.OrganizationID != principal.Organization.ID {
		return ErrPrincipalInvalid
	}

	organizationExists, err := tx.NewSelect().
		Model((*servermodels.Organization)(nil)).
		Where("id = ?", principal.Organization.ID).
		Exists(ctx)
	if err != nil {
		return err
	}
	if !organizationExists {
		return ErrPrincipalInvalid
	}

	userExists, err := tx.NewSelect().
		Model((*servermodels.User)(nil)).
		Where("id = ?", principal.User.ID).
		Where("organization_id = ?", principal.Organization.ID).
		Where("status = ?", "active").
		Exists(ctx)
	if err != nil {
		return err
	}
	if !userExists {
		return ErrPrincipalInvalid
	}
	return nil
}

func validateSourceChannel(ctx context.Context, tx bun.Tx, organizationID, channelID string) error {
	exists, err := tx.NewSelect().
		Model((*servermodels.Channel)(nil)).
		Where("organization_id = ?", organizationID).
		Where("id = ?", channelID).
		Where("deleted_at IS NULL").
		Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return &ValidationError{Fields: map[string]ValidationCode{"channelId": ValidationChannelInvalid}}
	}
	return nil
}

func replaceMethods(ctx context.Context, tx bun.Tx, organizationID, contactID string, methods []MethodInput) error {
	if _, err := tx.NewDelete().
		Model((*servermodels.ContactMethod)(nil)).
		Where("organization_id = ?", organizationID).
		Where("contact_id = ?", contactID).
		Exec(ctx); err != nil {
		return err
	}
	if len(methods) == 0 {
		return nil
	}

	records := make([]servermodels.ContactMethod, 0, len(methods))
	for _, method := range methods {
		record := servermodels.ContactMethod{
			OrganizationID:  organizationID,
			ContactID:       contactID,
			Type:            method.Type,
			Value:           method.Value,
			NormalizedValue: method.Value,
			IsPrimary:       method.IsPrimary,
		}
		if method.Label != "" {
			record.Label = &method.Label
		}
		records = append(records, record)
	}
	_, err := tx.NewInsert().
		Model(&records).
		Column("organization_id", "contact_id", "type", "value", "normalized_value", "label", "is_primary").
		Exec(ctx)
	return err
}

func loadContact(ctx context.Context, db bun.IDB, organizationID, contactID string, deleted bool) (*servermodels.Contact, error) {
	if !validUUID(contactID) {
		return nil, ErrNotFound
	}
	contact := &servermodels.Contact{}
	query := db.NewSelect().
		Model(contact).
		Where("c.id = ?", contactID).
		Where("c.organization_id = ?", organizationID)
	if deleted {
		query = query.Where("c.deleted_at IS NOT NULL")
	} else {
		query = query.Where("c.deleted_at IS NULL")
	}
	if err := query.Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return contact, nil
}
