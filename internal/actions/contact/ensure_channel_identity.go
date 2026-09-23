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
	// ExternalUserID 是验签通过的企业用户编号，非空时渠道身份关联到该编号的联系人。
	ExternalUserID string
	// Email 是签名身份中的规范化邮箱，非空时补充为联系人的联系方式。
	Email string
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
		// 读取渠道身份所属的同企业联系人。
		contact := &servermodels.Contact{}
		if err := db.NewSelect().
			Model(contact).
			Where("ct.organization_id = ?", input.OrganizationID).
			Where("ct.id = ?", identity.ContactID).
			Scan(ctx); err != nil {
			return EnsuredChannelIdentity{}, fmt.Errorf("load channel identity contact: %w", err)
		}
		if err := restoreContact(ctx, db, contact); err != nil {
			return EnsuredChannelIdentity{}, err
		}
		if err := addSignedEmail(ctx, db, contact, input.Email); err != nil {
			return EnsuredChannelIdentity{}, err
		}
		return EnsuredChannelIdentity{Contact: contact, Identity: identity}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return EnsuredChannelIdentity{}, fmt.Errorf("find channel identity: %w", err)
	}

	contact, err := ensureChannelContact(ctx, db, input)
	if err != nil {
		return EnsuredChannelIdentity{}, err
	}
	if err := addSignedEmail(ctx, db, contact, input.Email); err != nil {
		return EnsuredChannelIdentity{}, err
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

// ensureChannelContact 为新渠道身份取得联系人：带企业用户编号时复用该编号的联系人（含已软删除的），否则新建自动联系人。
// 并发首次写入同一企业用户编号时由唯一约束拒绝，调用方重试后读到已提交的联系人。
func ensureChannelContact(ctx context.Context, db bun.IDB, input EnsureChannelIdentityInput) (*servermodels.Contact, error) {
	if input.ExternalUserID != "" {
		contact := &servermodels.Contact{}
		err := db.NewSelect().
			Model(contact).
			Where("ct.organization_id = ?", input.OrganizationID).
			Where("ct.external_user_id = ?", input.ExternalUserID).
			For("UPDATE").
			Scan(ctx)
		if err == nil {
			return contact, restoreContact(ctx, db, contact)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("find contact by external user id: %w", err)
		}
	}
	contact := &servermodels.Contact{
		ID:              input.ContactID,
		OrganizationID:  input.OrganizationID,
		SourceChannelID: input.ChannelID,
		Stage:           string(domain.ContactStageVisitor),
	}
	if input.ExternalUserID != "" {
		contact.ExternalUserID = &input.ExternalUserID
	}
	if _, err := db.NewInsert().
		Model(contact).
		Column("id", "organization_id", "created_by_user_id", "source_channel_id", "display_name", "stage", "notes", "external_user_id").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create automatic contact: %w", err)
	}
	return contact, nil
}

// restoreContact 恢复已移入回收站的联系人。
func restoreContact(ctx context.Context, db bun.IDB, contact *servermodels.Contact) error {
	if contact.DeletedAt == nil {
		return nil
	}
	if _, err := db.NewUpdate().
		Model(contact).
		Set("deleted_at = NULL").
		Set("updated_at = now()").
		WherePK().
		Where("organization_id = ?", contact.OrganizationID).
		Exec(ctx); err != nil {
		return fmt.Errorf("restore automatic contact: %w", err)
	}
	contact.DeletedAt = nil
	return nil
}

// addSignedEmail 在联系人没有该邮箱时添加为联系方式，联系人没有主邮箱时设为主邮箱；已有邮箱不覆盖。
func addSignedEmail(ctx context.Context, db bun.IDB, contact *servermodels.Contact, address string) error {
	if address == "" {
		return nil
	}
	if _, err := db.NewRaw(`INSERT INTO contact_methods (organization_id, contact_id, type, value, normalized_value, is_primary)
		SELECT ?, ?, ?, ?, ?, NOT EXISTS (
			SELECT 1 FROM contact_methods WHERE contact_id = ? AND type = ? AND is_primary
		)
		ON CONFLICT DO NOTHING`,
		contact.OrganizationID, contact.ID, domain.ContactMethodTypeEmail, address, address,
		contact.ID, domain.ContactMethodTypeEmail,
	).Exec(ctx); err != nil {
		return fmt.Errorf("add signed contact email: %w", err)
	}
	return nil
}
