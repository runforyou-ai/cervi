//go:build server

package appservice

import (
	"fmt"
	"log/slog"

	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

func localizedError(meta RequestMeta, status int, code string, messageKey cervii18n.Key, fieldKeys map[string]cervii18n.Key) *Error {
	message, _ := cervii18n.Localize(string(meta.Locale), messageKey)
	return &Error{Status: status, Code: code, Message: message, Fields: cervii18n.LocalizeMap(string(meta.Locale), fieldKeys)}
}

func identityFromModel(identity *servermodels.Identity) Identity {
	return Identity{Organization: organizationFromModel(identity.Organization), User: userFromModel(identity.User)}
}

func organizationFromModel(organization servermodels.Organization) Organization {
	return Organization{ID: organization.ID, Name: organization.Name}
}

func userFromModel(user servermodels.User) User {
	return User{ID: user.ID, OrganizationID: user.OrganizationID, Email: user.Email, DisplayName: user.DisplayName, Role: UserRole(user.Role), Status: UserStatus(user.Status)}
}

func installationFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		installationaction.ValidationOrganizationNameRequired: cervii18n.FieldOrganizationNameRequired,
		installationaction.ValidationDisplayNameRequired:      cervii18n.FieldDisplayNameRequired,
		installationaction.ValidationEmailInvalid:             cervii18n.FieldEmailInvalid,
		installationaction.ValidationPasswordTooShort:         cervii18n.FieldPasswordTooShort,
		installationaction.ValidationPasswordTooLong:          cervii18n.FieldPasswordTooLong,
	}
	return translateValidationFields(fields, keys)
}

func translateValidationFields[Code comparable](fields map[string]Code, keys map[Code]cervii18n.Key) map[string]cervii18n.Key {
	result := make(map[string]cervii18n.Key, len(fields))
	for field, code := range fields {
		key, exists := keys[code]
		if !exists {
			slog.Warn("未映射的校验错误码", "field", field, "code", fmt.Sprint(code))
			continue
		}
		result[field] = key
	}
	return result
}
