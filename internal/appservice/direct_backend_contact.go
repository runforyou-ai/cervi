//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// ListContacts 返回联系人列表。
func (b *DirectBackend) ListContacts(ctx context.Context, meta RequestMeta, input ContactListInput) (ContactList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ContactList{}, err
	}
	output, err := b.listContacts.Execute(ctx, identity, contactaction.ListInput{
		Query: input.Query, Stage: optionalDomain[ContactStage, domain.ContactStage](input.Stage), ChannelID: input.ChannelID, MethodType: optionalDomain[ContactMethodType, domain.ContactMethodType](input.MethodType),
		Sort: domain.ContactSort(input.Sort), Page: input.Page, PageSize: input.PageSize, Deleted: input.Deleted,
	})
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return ContactList{}, localizedError(meta, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, contactFieldKeys(validationError.Fields))
	}
	if err != nil {
		return ContactList{}, b.contactError(ctx, meta, err, cervii18n.ErrorContactListFailed)
	}
	contacts := make([]ContactSummary, 0, len(output.Contacts))
	for _, contact := range output.Contacts {
		contacts = append(contacts, ContactSummary{
			ID: contact.ID, DisplayName: contact.DisplayName, Stage: ContactStage(contact.Stage), PrimaryEmail: contact.PrimaryEmail,
			PrimaryPhone: contact.PrimaryPhone, SourceChannelName: contact.SourceChannelName, CreatedAt: contact.CreatedAt, DeletedAt: contact.DeletedAt,
		})
	}
	return ContactList{Contacts: contacts, Page: PageInfo{Number: output.Page.Number, Size: output.Page.Size, Total: output.Page.Total}}, nil
}

// GetContact 返回联系人详情。
func (b *DirectBackend) GetContact(ctx context.Context, meta RequestMeta, contactID string) (Contact, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Contact{}, err
	}
	contact, err := b.getContact.Execute(ctx, identity, contactID)
	if err != nil {
		return Contact{}, b.contactError(ctx, meta, err, cervii18n.ErrorContactReadFailed)
	}
	return contactFromAction(contact), nil
}

// CreateContact 创建联系人。
func (b *DirectBackend) CreateContact(ctx context.Context, meta RequestMeta, input ContactInput) (Contact, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Contact{}, err
	}
	contact, err := b.createContact.Execute(ctx, identity, contactInput(input))
	if err != nil {
		return Contact{}, b.contactMutationError(ctx, meta, err, cervii18n.ErrorContactCreateFailed)
	}
	slog.Info("联系人创建成功", "organization_id", identity.Organization.ID, "contact_id", contact.Contact.ID)
	return contactFromAction(contact), nil
}

// UpdateContact 修改联系人。
func (b *DirectBackend) UpdateContact(ctx context.Context, meta RequestMeta, contactID string, input ContactInput) (Contact, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Contact{}, err
	}
	contact, err := b.updateContact.Execute(ctx, identity, contactID, contactInput(input))
	if err != nil {
		return Contact{}, b.contactMutationError(ctx, meta, err, cervii18n.ErrorContactUpdateFailed)
	}
	slog.Info("联系人更新成功", "organization_id", identity.Organization.ID, "contact_id", contact.Contact.ID)
	return contactFromAction(contact), nil
}

// DeleteContact 将联系人移入回收站。
func (b *DirectBackend) DeleteContact(ctx context.Context, meta RequestMeta, contactID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteContact.Execute(ctx, identity, contactID); err != nil {
		return b.contactError(ctx, meta, err, cervii18n.ErrorContactDeleteFailed)
	}
	slog.Info("联系人移入回收站", "organization_id", identity.Organization.ID, "contact_id", contactID)
	return nil
}

// RestoreContact 恢复联系人。
func (b *DirectBackend) RestoreContact(ctx context.Context, meta RequestMeta, contactID string) (Contact, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Contact{}, err
	}
	contact, err := b.restoreContact.Execute(ctx, identity, contactID)
	if err != nil {
		return Contact{}, b.contactError(ctx, meta, err, cervii18n.ErrorContactRestoreFailed)
	}
	slog.Info("联系人恢复成功", "organization_id", identity.Organization.ID, "contact_id", contact.Contact.ID)
	return contactFromAction(contact), nil
}

func (b *DirectBackend) contactMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return localizedError(meta, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, contactFieldKeys(validationError.Fields))
	}
	return b.contactError(ctx, meta, err, failureKey)
}

func (b *DirectBackend) contactError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return localizedError(meta, http.StatusUnauthorized, "AUTH_REQUIRED", cervii18n.ErrorAuthenticationRequired, nil)
	}
	if errors.Is(err, contactaction.ErrNotFound) {
		return localizedError(meta, http.StatusNotFound, "CONTACT_NOT_FOUND", cervii18n.ErrorContactNotFound, nil)
	}
	slog.Warn("联系人操作失败", "failure", failureKey, "error", err)
	return localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", failureKey, nil)
}

func contactInput(input ContactInput) contactaction.ContactInput {
	methods := make([]contactaction.MethodInput, 0, len(input.Methods))
	for _, method := range input.Methods {
		methods = append(methods, contactaction.MethodInput{Type: domain.ContactMethodType(method.Type), Value: method.Value, Label: method.Label, IsPrimary: method.IsPrimary})
	}
	return contactaction.ContactInput{DisplayName: input.DisplayName, ChannelID: input.ChannelID, Stage: domain.ContactStage(input.Stage), Notes: input.Notes, Methods: methods}
}

func contactFromAction(contact *contactaction.ContactDetail) Contact {
	methods := make([]ContactMethod, 0, len(contact.Methods))
	for _, method := range contact.Methods {
		methods = append(methods, ContactMethod{Type: ContactMethodType(method.Type), Value: method.Value, Label: method.Label, IsPrimary: method.IsPrimary})
	}
	identities := make([]ContactChannelIdentity, 0, len(contact.ChannelIdentities))
	for _, identity := range contact.ChannelIdentities {
		identities = append(identities, ContactChannelIdentity{
			ChannelID: identity.ChannelID, ChannelName: identity.ChannelName, ExternalID: identity.ExternalID, DisplayName: identity.DisplayName,
		})
	}
	return Contact{
		Contact: ContactRecord{
			ID: contact.Contact.ID, SourceChannelID: contact.Contact.SourceChannelID, DisplayName: contact.Contact.DisplayName,
			Stage: ContactStage(contact.Contact.Stage), Notes: contact.Contact.Notes, CreatedAt: contact.Contact.CreatedAt,
		},
		SourceChannel: ContactSourceChannel{ID: contact.SourceChannel.ID, Type: ChannelType(contact.SourceChannel.Type), Name: contact.SourceChannel.Name},
		Methods:       methods, ChannelIdentities: identities,
	}
}

func contactFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		contactaction.ValidationIdentityRequired: cervii18n.FieldContactIdentityRequired, contactaction.ValidationChannelRequired: cervii18n.FieldContactChannelRequired,
		contactaction.ValidationChannelInvalid: cervii18n.FieldContactChannelInvalid, contactaction.ValidationChannelImmutable: cervii18n.FieldContactChannelImmutable,
		contactaction.ValidationNameTooLong: cervii18n.FieldContactNameTooLong, contactaction.ValidationStageInvalid: cervii18n.FieldContactStageInvalid,
		contactaction.ValidationNotesTooLong: cervii18n.FieldContactNotesTooLong, contactaction.ValidationMethodsTooMany: cervii18n.FieldContactMethodsTooMany,
		contactaction.ValidationMethodInvalid: cervii18n.FieldContactMethodInvalid, contactaction.ValidationMethodDuplicate: cervii18n.FieldContactMethodDuplicate,
		contactaction.ValidationPrimaryDuplicate: cervii18n.FieldContactPrimaryDuplicate, contactaction.ValidationQueryInvalid: cervii18n.FieldContactQueryInvalid,
	}
	return translateValidationFields(fields, keys)
}
