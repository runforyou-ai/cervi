//go:build server

package appservice

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

const localObjectBaseURL = "/storage"

// identityFromModel 把存储身份转换为应用契约并补齐文件地址。
func (b *DirectBackend) identityFromModel(ctx context.Context, identity *servermodels.Identity) (Identity, error) {
	user, err := b.currentUserFromIdentity(ctx, identity)
	if err != nil {
		return Identity{}, err
	}
	return Identity{Organization: organizationFromModel(identity.Organization), User: user}, nil
}

// organizationFromModel 把存储企业转换为应用契约。
func organizationFromModel(organization servermodels.Organization) Organization {
	return Organization{
		ID: organization.ID, Name: organization.Name, AllowArbitraryURL: organization.AllowArbitraryURL,
	}
}

// currentUserFromIdentity 把存储身份转换为当前用户契约并补齐头像地址。
func (b *DirectBackend) currentUserFromIdentity(ctx context.Context, identity *servermodels.Identity) (CurrentUser, error) {
	user := currentUserContract(identity)
	fileID := identity.OrganizationIdentity.AvatarFileID
	if fileID == nil || *fileID == "" {
		return user, nil
	}
	urls, err := b.activeFileURLs(ctx, identity, []string{*fileID})
	if err != nil {
		return CurrentUser{}, err
	}
	user.AvatarURL = urls[*fileID]
	return user, nil
}

// currentUserContract 执行不访问存储的当前用户契约转换。
func currentUserContract(identity *servermodels.Identity) CurrentUser {
	user := identity.User
	organizationIdentity := identity.OrganizationIdentity
	return CurrentUser{
		ID: user.ID, IdentityID: user.IdentityID, OrganizationID: user.OrganizationID, Email: user.Email, DisplayName: organizationIdentity.DisplayName,
		RoleID: organizationIdentity.RoleID, Status: UserStatus(user.Status), Locale: Locale(user.Locale), TimeZone: user.TimeZone, MessageNotificationsEnabled: user.MessageNotificationsEnabled, WorkspaceTabsEnabled: user.WorkspaceTabsEnabled,
		WorkStatus: WorkStatus(organizationIdentity.WorkStatus),
	}
}

// activeFileURLs 批量解析当前企业已关联文件的公开地址。
func (b *DirectBackend) activeFileURLs(ctx context.Context, identity *servermodels.Identity, fileIDs []string) (map[string]string, error) {
	locations, err := b.getFile.ListActiveLocations(ctx, identity, fileIDs)
	if err != nil {
		return nil, err
	}
	publicBaseURL := ""
	for _, location := range locations {
		if location.StorageBackend == domain.FileStorageBackendS3 {
			setting, err := b.getS3Setting.Execute(ctx, identity)
			if err != nil {
				return nil, err
			}
			publicBaseURL = setting.PublicBaseURL
			break
		}
	}
	urls := make(map[string]string, len(locations))
	for _, location := range locations {
		contentURL, err := fileContentURL(location.StorageBackend, location.StorageKey, publicBaseURL)
		if err != nil {
			return nil, fmt.Errorf("build public URL for file %s: %w", location.ID, err)
		}
		urls[location.ID] = contentURL
	}
	return urls, nil
}

// fileContentURL 按文件实际存储类型生成稳定公开地址。
func fileContentURL(backend domain.FileStorageBackend, storageKey, publicBaseURL string) (string, error) {
	baseURL := localObjectBaseURL
	if backend == domain.FileStorageBackendS3 {
		baseURL = publicBaseURL
	} else if backend != domain.FileStorageBackendLocal {
		return "", fmt.Errorf("unsupported file storage backend %q", backend)
	}
	return serverfilecontent.PublicURL(baseURL, storageKey)
}

// optionalFileURL 返回可选文件编号对应的公开地址。
func optionalFileURL(urls map[string]string, fileID *string) string {
	if fileID == nil {
		return ""
	}
	return urls[*fileID]
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
