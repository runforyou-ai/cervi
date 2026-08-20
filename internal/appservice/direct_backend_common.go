//go:build server

package appservice

import (
	"fmt"
	"log/slog"

	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// localizedError 把错误码转换为本地化业务错误。
func localizedError(meta RequestMeta, status int, code string, messageKey cervii18n.Key, fieldKeys map[string]cervii18n.Key) *Error {
	message, _ := cervii18n.Localize(string(meta.Locale), messageKey)
	return &Error{Status: status, Code: code, Message: message, Fields: cervii18n.LocalizeMap(string(meta.Locale), fieldKeys)}
}

// identityFromModel 把存储身份转换为应用契约。
func identityFromModel(identity *servermodels.Identity) Identity {
	return Identity{Organization: organizationFromModel(identity.Organization), User: userFromModel(identity.User)}
}

// organizationFromModel 把存储企业转换为应用契约。
func organizationFromModel(organization servermodels.Organization) Organization {
	return Organization{ID: organization.ID, Name: organization.Name}
}

// userFromModel 把存储用户转换为应用契约。
func userFromModel(user servermodels.User) User {
	return User{ID: user.ID, OrganizationID: user.OrganizationID, Email: user.Email, DisplayName: user.DisplayName, Role: UserRole(user.Role), Status: UserStatus(user.Status)}
}

// translateValidationFields 把校验错误码映射为本地化文案键。
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

// optionalDomain 把可选枚举指针转换为领域值，缺省为空。
func optionalDomain[T ~string, D ~string](value *T) D {
	if value == nil {
		return ""
	}
	return D(*value)
}
