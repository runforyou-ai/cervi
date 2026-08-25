//go:build server

package contact

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// EnsureChannelIdentityInput 定义渠道自动联系人所需的稳定标识。
type EnsureChannelIdentityInput struct {
	OrganizationID string
	ChannelID      string
	ExternalID     string
	ContactID      string
	IdentityID     string
}

// EnsuredChannelIdentity 返回联系人和渠道身份。
type EnsuredChannelIdentity struct {
	Contact  *servermodels.Contact
	Identity *servermodels.ContactChannelIdentity
}

// EnsureChannelIdentity 在调用方事务中取得或创建联系人渠道身份。
func EnsureChannelIdentity(ctx context.Context, db bun.IDB, input EnsureChannelIdentityInput) (EnsuredChannelIdentity, error) {
	identity := &servermodels.ContactChannelIdentity{}
	err := db.NewSelect().
		Model(identity).
		Where("cci.organization_id = ?", input.OrganizationID).
		Where("cci.channel_id = ?", input.ChannelID).
		Where("cci.external_id = ?", input.ExternalID).
		For("UPDATE").
		Scan(ctx)
	if err == nil {
		contact, loadErr := loadAutomaticContact(ctx, db, input.OrganizationID, identity.ContactID)
		if loadErr != nil {
			return EnsuredChannelIdentity{}, loadErr
		}
		if contact.DeletedAt != nil {
			if _, updateErr := db.NewUpdate().
				Model(contact).
				Set("deleted_at = NULL").
				Set("updated_at = now()").
				WherePK().
				Where("organization_id = ?", input.OrganizationID).
				Exec(ctx); updateErr != nil {
				return EnsuredChannelIdentity{}, fmt.Errorf("restore automatic contact: %w", updateErr)
			}
			contact.DeletedAt = nil
		}
		return EnsuredChannelIdentity{Contact: contact, Identity: identity}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return EnsuredChannelIdentity{}, fmt.Errorf("find channel identity: %w", err)
	}

	contact := &servermodels.Contact{
		ID:              input.ContactID,
		OrganizationID:  input.OrganizationID,
		SourceChannelID: input.ChannelID,
		Stage:           string(domain.ContactStageVisitor),
	}
	if _, err := db.NewInsert().
		Model(contact).
		Column("id", "organization_id", "created_by_user_id", "source_channel_id", "display_name", "stage", "notes").
		Exec(ctx); err != nil {
		return EnsuredChannelIdentity{}, fmt.Errorf("create automatic contact: %w", err)
	}
	identity = &servermodels.ContactChannelIdentity{
		ID:             input.IdentityID,
		OrganizationID: input.OrganizationID,
		ContactID:      contact.ID,
		ChannelID:      input.ChannelID,
		ExternalID:     input.ExternalID,
	}
	if _, err := db.NewInsert().
		Model(identity).
		Column("id", "organization_id", "contact_id", "channel_id", "external_id", "display_name").
		Exec(ctx); err != nil {
		return EnsuredChannelIdentity{}, fmt.Errorf("create channel identity: %w", err)
	}
	return EnsuredChannelIdentity{Contact: contact, Identity: identity}, nil
}

// loadAutomaticContact 读取渠道身份所属的同企业联系人。
func loadAutomaticContact(ctx context.Context, db bun.IDB, organizationID, contactID string) (*servermodels.Contact, error) {
	contact := &servermodels.Contact{}
	if err := db.NewSelect().
		Model(contact).
		Where("c.organization_id = ?", organizationID).
		Where("c.id = ?", contactID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load channel identity contact: %w", err)
	}
	return contact, nil
}
