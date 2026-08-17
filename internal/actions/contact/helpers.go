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
	existing := make([]servermodels.ContactMethod, 0)
	if err := tx.NewSelect().
		Model(&existing).
		Where("cm.organization_id = ?", organizationID).
		Where("cm.contact_id = ?", contactID).
		For("UPDATE").
		Scan(ctx); err != nil {
		return err
	}

	key := func(methodType, value string) string {
		return methodType + "\x00" + value
	}
	desired := make(map[string]MethodInput, len(methods))
	for _, method := range methods {
		desired[key(method.Type, method.Value)] = method
	}

	matched := make(map[string]*servermodels.ContactMethod, len(existing))
	obsoleteIDs := make([]string, 0)
	for index := range existing {
		record := &existing[index]
		recordKey := key(record.Type, record.NormalizedValue)
		if _, wanted := desired[recordKey]; wanted && matched[recordKey] == nil {
			matched[recordKey] = record
			continue
		}
		obsoleteIDs = append(obsoleteIDs, record.ID)
	}
	if len(obsoleteIDs) > 0 {
		if _, err := tx.NewDelete().
			Model((*servermodels.ContactMethod)(nil)).
			Where("organization_id = ?", organizationID).
			Where("contact_id = ?", contactID).
			Where("id IN (?)", bun.In(obsoleteIDs)).
			Exec(ctx); err != nil {
			return err
		}
	}

	// 先取消不再作为主要项的旧记录，避免同类型主要项切换时触发唯一索引冲突。
	for recordKey, record := range matched {
		if record.IsPrimary && !desired[recordKey].IsPrimary {
			if _, err := tx.NewUpdate().
				Model((*servermodels.ContactMethod)(nil)).
				Set("is_primary = false").
				Set("updated_at = now()").
				Where("organization_id = ?", organizationID).
				Where("contact_id = ?", contactID).
				Where("id = ?", record.ID).
				Exec(ctx); err != nil {
				return err
			}
			record.IsPrimary = false
		}
	}

	newRecords := make([]servermodels.ContactMethod, 0)
	for _, method := range methods {
		record := matched[key(method.Type, method.Value)]
		if record == nil {
			newRecord := servermodels.ContactMethod{
				OrganizationID:  organizationID,
				ContactID:       contactID,
				Type:            method.Type,
				Value:           method.Value,
				NormalizedValue: method.Value,
				IsPrimary:       method.IsPrimary,
			}
			if method.Label != "" {
				newRecord.Label = &method.Label
			}
			newRecords = append(newRecords, newRecord)
			continue
		}

		labelMatches := (record.Label == nil && method.Label == "") ||
			(record.Label != nil && *record.Label == method.Label)
		if labelMatches && record.IsPrimary == method.IsPrimary {
			continue
		}
		var label *string
		if method.Label != "" {
			label = &method.Label
		}
		if _, err := tx.NewUpdate().
			Model((*servermodels.ContactMethod)(nil)).
			Set("label = ?", label).
			Set("is_primary = ?", method.IsPrimary).
			Set("updated_at = now()").
			Where("organization_id = ?", organizationID).
			Where("contact_id = ?", contactID).
			Where("id = ?", record.ID).
			Exec(ctx); err != nil {
			return err
		}
	}
	if len(newRecords) == 0 {
		return nil
	}
	_, err := tx.NewInsert().
		Model(&newRecords).
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
