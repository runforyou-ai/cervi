//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

func (b *DirectBackend) contactMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *contactaction.ValidationError
	if errors.As(err, &validationError) {
		return localizedError(meta, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, contactFieldKeys(validationError.Fields))
	}
	return b.contactError(ctx, meta, err, failureKey)
}

func (b *DirectBackend) contactError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, contactaction.ErrPrincipalInvalid) {
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
		methods = append(methods, contactaction.MethodInput{Type: string(method.Type), Value: method.Value, Label: method.Label, IsPrimary: method.IsPrimary})
	}
	return contactaction.ContactInput{DisplayName: input.DisplayName, ChannelID: input.ChannelID, Stage: string(input.Stage), Notes: input.Notes, Methods: methods}
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

func contactFieldKeys(fields map[string]contactaction.ValidationCode) map[string]cervii18n.Key {
	keys := map[contactaction.ValidationCode]cervii18n.Key{
		contactaction.ValidationIdentityRequired: cervii18n.FieldContactIdentityRequired, contactaction.ValidationChannelRequired: cervii18n.FieldContactChannelRequired,
		contactaction.ValidationChannelInvalid: cervii18n.FieldContactChannelInvalid, contactaction.ValidationChannelImmutable: cervii18n.FieldContactChannelImmutable,
		contactaction.ValidationNameTooLong: cervii18n.FieldContactNameTooLong, contactaction.ValidationStageInvalid: cervii18n.FieldContactStageInvalid,
		contactaction.ValidationNotesTooLong: cervii18n.FieldContactNotesTooLong, contactaction.ValidationMethodsTooMany: cervii18n.FieldContactMethodsTooMany,
		contactaction.ValidationMethodInvalid: cervii18n.FieldContactMethodInvalid, contactaction.ValidationMethodDuplicate: cervii18n.FieldContactMethodDuplicate,
		contactaction.ValidationPrimaryDuplicate: cervii18n.FieldContactPrimaryDuplicate, contactaction.ValidationQueryInvalid: cervii18n.FieldContactQueryInvalid,
	}
	return translateValidationFields(fields, keys)
}
