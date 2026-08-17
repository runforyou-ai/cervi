//go:build server

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

type memoryApplication struct {
	installed            bool
	principal            servermodels.Principal
	password             string
	sessions             map[string]time.Time
	channels             map[string]servermodels.Channel
	contacts             map[string]contactaction.ContactDetail
	contactOrganizations map[string]string
	contactDeletedAt     map[string]*time.Time
	settings             map[string]servermodels.WebsiteChannelSetting
	nextID               int
	deleteErr            error
}

// newMemoryApplication 创建未初始化的内存测试应用。
func newMemoryApplication() *memoryApplication {
	return &memoryApplication{
		sessions:             make(map[string]time.Time),
		channels:             make(map[string]servermodels.Channel),
		contacts:             make(map[string]contactaction.ContactDetail),
		contactOrganizations: make(map[string]string),
		contactDeletedAt:     make(map[string]*time.Time),
		settings:             make(map[string]servermodels.WebsiteChannelSetting),
	}
}

// newTestService 使用内存执行函数组装测试 API。
func newTestService(application *memoryApplication) *Service {
	return NewService(Dependencies{
		InstallWorkspace:                  application.install,
		Login:                             application.login,
		Logout:                            application.logout,
		ResolveSession:                    application.resolveSession,
		Installation:                      application.installationStatus,
		LoadInbox:                         application.loadInbox,
		ListWebsiteChannels:               application.listWebsiteChannels,
		GetWebsiteChannel:                 application.getWebsiteChannel,
		CreateWebsiteChannel:              application.createWebsiteChannel,
		UpdateWebsiteChannel:              application.updateWebsiteChannel,
		UpdateWebsiteChannelChatInterface: application.updateWebsiteChannelChatInterface,
		DeleteWebsiteChannel:              application.deleteWebsiteChannel,
		RestoreWebsiteChannel:             application.restoreWebsiteChannel,
		ListChannels:                      application.listChannels,
		ListUsers:                         application.listUsers,
		GetUser:                           application.getUser,
		ListContacts:                      application.listContacts,
		GetContact:                        application.getContact,
		CreateContact:                     application.createContact,
		UpdateContact:                     application.updateContact,
		DeleteContact:                     application.deleteContact,
		RestoreContact:                    application.restoreContact,
	})
}

// install 在内存中创建企业所有者和初始会话。
func (a *memoryApplication) install(_ context.Context, input installationaction.InstallWorkspaceInput) (installationaction.InstallWorkspaceOutput, error) {
	if a.installed {
		return installationaction.InstallWorkspaceOutput{}, installationaction.ErrAlreadyInstalled
	}
	a.installed = true
	a.password = input.Password
	a.principal = servermodels.Principal{
		Organization: servermodels.Organization{ID: "organization-1", Name: input.OrganizationName},
		User: servermodels.User{
			ID:             "user-1",
			OrganizationID: "organization-1",
			Email:          strings.ToLower(input.Email),
			DisplayName:    input.DisplayName,
			Role:           "owner",
			Status:         "active",
		},
	}
	expiresAt := time.Now().Add(time.Hour)
	a.sessions["install-token"] = expiresAt
	principal := a.principal
	return installationaction.InstallWorkspaceOutput{
		Principal: &principal,
		Token:     "install-token",
		ExpiresAt: expiresAt,
	}, nil
}

// login 在内存中校验测试账号并创建会话。
func (a *memoryApplication) login(_ context.Context, input authaction.LoginInput) (authaction.LoginOutput, error) {
	if !a.installed || !strings.EqualFold(strings.TrimSpace(input.Email), a.principal.User.Email) || input.Password != a.password {
		return authaction.LoginOutput{}, authaction.ErrInvalidCredentials
	}
	expiresAt := time.Now().Add(time.Hour)
	a.sessions["login-token"] = expiresAt
	principal := a.principal
	return authaction.LoginOutput{Principal: &principal, Token: "login-token", ExpiresAt: expiresAt}, nil
}

// logout 在内存中删除测试会话。
func (a *memoryApplication) logout(_ context.Context, token string) error {
	if a.deleteErr != nil {
		return a.deleteErr
	}
	delete(a.sessions, token)
	return nil
}

// resolveSession 查找有效测试会话对应的用户身份。
func (a *memoryApplication) resolveSession(_ context.Context, token string) (*servermodels.Principal, error) {
	expiresAt, exists := a.sessions[token]
	if !exists || !expiresAt.After(time.Now()) {
		return nil, nil
	}
	principal := a.principal
	return &principal, nil
}

// installationStatus 返回内存测试应用的安装状态。
func (a *memoryApplication) installationStatus(context.Context) (bool, error) {
	return a.installed, nil
}

// loadInbox 返回内存测试用户的空收件箱。
func (a *memoryApplication) loadInbox(_ context.Context, principal *servermodels.Principal) inboxaction.LoadInboxOutput {
	return inboxaction.LoadInboxOutput{
		Organization:  principal.Organization,
		User:          principal.User,
		Conversations: []any{},
	}
}

// listWebsiteChannels 按删除状态返回当前企业的网站渠道。
func (a *memoryApplication) listWebsiteChannels(_ context.Context, principal *servermodels.Principal, deleted bool) ([]servermodels.Channel, error) {
	channels := make([]servermodels.Channel, 0)
	for _, item := range a.channels {
		if item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite {
			continue
		}
		if (item.DeletedAt != nil) != deleted {
			continue
		}
		channels = append(channels, item)
	}
	return channels, nil
}

// getWebsiteChannel 返回当前企业中未删除的网站渠道。
func (a *memoryApplication) getWebsiteChannel(_ context.Context, principal *servermodels.Principal, channelID string) (*channelaction.WebsiteChannelDetail, error) {
	item, exists := a.channels[channelID]
	if !exists || item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite || item.DeletedAt != nil {
		return nil, channelaction.ErrNotFound
	}
	return &channelaction.WebsiteChannelDetail{Channel: &item, ChatInterface: a.settings[channelID]}, nil
}

// createWebsiteChannel 创建内存网站渠道。
func (a *memoryApplication) createWebsiteChannel(_ context.Context, principal *servermodels.Principal, input channelaction.WebsiteChannelInput) (*servermodels.Channel, error) {
	a.nextID++
	now := time.Now()
	item := servermodels.Channel{
		ID:              "channel-" + strconv.Itoa(a.nextID),
		OrganizationID:  principal.Organization.ID,
		CreatedByUserID: principal.User.ID,
		Type:            channelaction.TypeWebsite,
		Name:            strings.TrimSpace(input.Name),
		DefaultLocale:   input.DefaultLocale,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if description := strings.TrimSpace(input.Description); description != "" {
		item.Description = &description
	}
	a.channels[item.ID] = item
	a.settings[item.ID] = servermodels.WebsiteChannelSetting{
		ChannelID:      item.ID,
		OrganizationID: item.OrganizationID,
		ChatTitle:      item.Name,
		ThemeColor:     channelaction.DefaultWebsiteChannelThemeColor,
	}
	return &item, nil
}

// updateWebsiteChannel 修改内存网站渠道。
func (a *memoryApplication) updateWebsiteChannel(_ context.Context, principal *servermodels.Principal, channelID string, input channelaction.WebsiteChannelInput) (*servermodels.Channel, error) {
	item, exists := a.channels[channelID]
	if !exists || item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite || item.DeletedAt != nil {
		return nil, channelaction.ErrNotFound
	}
	item.Name = strings.TrimSpace(input.Name)
	item.Description = nil
	if description := strings.TrimSpace(input.Description); description != "" {
		item.Description = &description
	}
	item.DefaultLocale = input.DefaultLocale
	item.UpdatedAt = time.Now()
	a.channels[channelID] = item
	return &item, nil
}

// updateWebsiteChannelChatInterface 修改内存网站渠道聊天界面。
func (a *memoryApplication) updateWebsiteChannelChatInterface(_ context.Context, principal *servermodels.Principal, channelID string, input channelaction.WebsiteChannelChatInterfaceInput) (*servermodels.WebsiteChannelSetting, error) {
	item, exists := a.channels[channelID]
	if !exists || item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite || item.DeletedAt != nil {
		return nil, channelaction.ErrNotFound
	}
	setting := a.settings[channelID]
	setting.ChatTitle = strings.TrimSpace(input.Title)
	setting.ChatSubtitle = nil
	if subtitle := strings.TrimSpace(input.Subtitle); subtitle != "" {
		setting.ChatSubtitle = &subtitle
	}
	setting.GreetingMessage = nil
	if greeting := strings.TrimSpace(input.GreetingMessage); greeting != "" {
		setting.GreetingMessage = &greeting
	}
	setting.ThemeColor = strings.ToUpper(strings.TrimSpace(input.ThemeColor))
	a.settings[channelID] = setting
	return &setting, nil
}

// deleteWebsiteChannel 软删除内存网站渠道。
func (a *memoryApplication) deleteWebsiteChannel(_ context.Context, principal *servermodels.Principal, channelID string) error {
	item, exists := a.channels[channelID]
	if !exists || item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite || item.DeletedAt != nil {
		return channelaction.ErrNotFound
	}
	now := time.Now()
	item.DeletedAt = &now
	item.UpdatedAt = now
	a.channels[channelID] = item
	return nil
}

// restoreWebsiteChannel 恢复内存网站渠道。
func (a *memoryApplication) restoreWebsiteChannel(_ context.Context, principal *servermodels.Principal, channelID string) (*servermodels.Channel, error) {
	item, exists := a.channels[channelID]
	if !exists || item.OrganizationID != principal.Organization.ID || item.Type != channelaction.TypeWebsite || item.DeletedAt == nil {
		return nil, channelaction.ErrNotFound
	}
	item.DeletedAt = nil
	item.UpdatedAt = time.Now()
	a.channels[channelID] = item
	return &item, nil
}

// listChannels 返回当前企业的有效渠道摘要。
func (a *memoryApplication) listChannels(_ context.Context, principal *servermodels.Principal) ([]channelaction.Summary, error) {
	channels := make([]channelaction.Summary, 0)
	for _, item := range a.channels {
		if item.OrganizationID == principal.Organization.ID && item.DeletedAt == nil {
			channels = append(channels, channelaction.Summary{ID: item.ID, Type: item.Type, Name: item.Name})
		}
	}
	return channels, nil
}

// listUsers 返回当前企业的内存团队成员。
func (a *memoryApplication) listUsers(_ context.Context, principal *servermodels.Principal, input useraction.ListInput) (useraction.ListOutput, error) {
	users := []useraction.DirectoryUser{memoryDirectoryUser(principal.User)}
	return useraction.ListOutput{Users: users, Page: useraction.PageInfo{Number: input.Page, Size: input.PageSize, Total: 1}}, nil
}

// getUser 返回当前企业的内存团队成员详情。
func (a *memoryApplication) getUser(_ context.Context, principal *servermodels.Principal, userID string) (*useraction.DirectoryUser, error) {
	if userID != principal.User.ID {
		return nil, useraction.ErrNotFound
	}
	user := memoryDirectoryUser(principal.User)
	return &user, nil
}

// memoryDirectoryUser 创建内存团队成员。
func memoryDirectoryUser(user servermodels.User) useraction.DirectoryUser {
	return useraction.DirectoryUser{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Status:      user.Status,
		CreatedAt:   time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
}

// listContacts 返回内存联系人列表。
func (a *memoryApplication) listContacts(_ context.Context, principal *servermodels.Principal, input contactaction.ListInput) (contactaction.ListOutput, error) {
	contacts := make([]contactaction.ContactSummary, 0)
	for _, detail := range a.contacts {
		item := detail.Contact
		deletedAt := a.contactDeletedAt[item.ID]
		if a.contactOrganizations[item.ID] != principal.Organization.ID || (deletedAt != nil) != input.Deleted {
			continue
		}
		summary := contactaction.ContactSummary{
			ID: item.ID, DisplayName: item.DisplayName, Stage: item.Stage,
			CreatedAt: item.CreatedAt, DeletedAt: deletedAt,
		}
		for _, method := range detail.Methods {
			value := method.Value
			if method.Type == contactaction.MethodEmail && summary.PrimaryEmail == nil {
				summary.PrimaryEmail = &value
			}
			if method.Type == contactaction.MethodPhone && summary.PrimaryPhone == nil {
				summary.PrimaryPhone = &value
			}
		}
		contacts = append(contacts, summary)
	}
	return contactaction.ListOutput{
		Contacts: contacts,
		Page:     contactaction.PageInfo{Number: input.Page, Size: input.PageSize, Total: len(contacts)},
	}, nil
}

// getContact 返回内存联系人详情。
func (a *memoryApplication) getContact(_ context.Context, principal *servermodels.Principal, contactID string) (*contactaction.ContactDetail, error) {
	detail, exists := a.contacts[contactID]
	if !exists || a.contactOrganizations[contactID] != principal.Organization.ID || a.contactDeletedAt[contactID] != nil {
		return nil, contactaction.ErrNotFound
	}
	return &detail, nil
}

// createContact 创建内存联系人。
func (a *memoryApplication) createContact(_ context.Context, principal *servermodels.Principal, input contactaction.ContactInput) (*contactaction.ContactDetail, error) {
	a.nextID++
	id := "00000000-0000-0000-0000-" + fmt.Sprintf("%012d", a.nextID)
	now := time.Now()
	contact := contactaction.ContactRecord{
		ID:              id,
		SourceChannelID: input.ChannelID, Stage: input.Stage, CreatedAt: now,
	}
	if input.DisplayName != "" {
		contact.DisplayName = &input.DisplayName
	}
	if input.Notes != "" {
		contact.Notes = &input.Notes
	}
	detail := contactaction.ContactDetail{Contact: contact, Methods: memoryContactMethods(input.Methods)}
	a.contacts[id] = detail
	a.contactOrganizations[id] = principal.Organization.ID
	return &detail, nil
}

// updateContact 修改内存联系人。
func (a *memoryApplication) updateContact(_ context.Context, principal *servermodels.Principal, contactID string, input contactaction.ContactInput) (*contactaction.ContactDetail, error) {
	detail, exists := a.contacts[contactID]
	if !exists || a.contactOrganizations[contactID] != principal.Organization.ID || a.contactDeletedAt[contactID] != nil {
		return nil, contactaction.ErrNotFound
	}
	detail.Contact.DisplayName = nil
	detail.Contact.Notes = nil
	if input.DisplayName != "" {
		detail.Contact.DisplayName = &input.DisplayName
	}
	if input.Notes != "" {
		detail.Contact.Notes = &input.Notes
	}
	detail.Contact.Stage = input.Stage
	detail.Contact.SourceChannelID = input.ChannelID
	detail.Methods = memoryContactMethods(input.Methods)
	a.contacts[contactID] = detail
	return &detail, nil
}

// deleteContact 软删除内存联系人。
func (a *memoryApplication) deleteContact(_ context.Context, principal *servermodels.Principal, contactID string) error {
	_, exists := a.contacts[contactID]
	if !exists || a.contactOrganizations[contactID] != principal.Organization.ID || a.contactDeletedAt[contactID] != nil {
		return contactaction.ErrNotFound
	}
	now := time.Now()
	a.contactDeletedAt[contactID] = &now
	return nil
}

// restoreContact 恢复内存联系人。
func (a *memoryApplication) restoreContact(_ context.Context, principal *servermodels.Principal, contactID string) (*contactaction.ContactDetail, error) {
	detail, exists := a.contacts[contactID]
	if !exists || a.contactOrganizations[contactID] != principal.Organization.ID || a.contactDeletedAt[contactID] == nil {
		return nil, contactaction.ErrNotFound
	}
	a.contactDeletedAt[contactID] = nil
	return &detail, nil
}

// memoryContactMethods 创建内存联系方式。
func memoryContactMethods(inputs []contactaction.MethodInput) []contactaction.ContactMethod {
	methods := make([]contactaction.ContactMethod, 0, len(inputs))
	for _, input := range inputs {
		methods = append(methods, contactaction.ContactMethod{
			Type: input.Type, Value: input.Value, IsPrimary: input.IsPrimary,
		})
	}
	return methods
}

// TestInstallationAndAuthenticationFlow 验证安装、登录和登出的完整 HTTP 流程。
func TestInstallationAndAuthenticationFlow(t *testing.T) {
	application := newMemoryApplication()
	server := httptest.NewServer(newTestService(application))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	assertErrorCode(t, doJSON(t, client, http.MethodGet, server.URL+"/inbox", nil), http.StatusConflict, "INSTALLATION_REQUIRED")

	installResponse := doJSON(t, client, http.MethodPost, server.URL+"/install", map[string]string{
		"organizationName": "鹿行测试公司",
		"displayName":      "所有者",
		"email":            "owner@example.com",
		"password":         "password123",
	})
	if installResponse.StatusCode != http.StatusCreated {
		t.Fatalf("install status = %d, want %d", installResponse.StatusCode, http.StatusCreated)
	}
	installResponse.Body.Close()

	inboxResponse := doJSON(t, client, http.MethodGet, server.URL+"/inbox", nil)
	if inboxResponse.StatusCode != http.StatusOK {
		t.Fatalf("authenticated inbox status = %d, want %d", inboxResponse.StatusCode, http.StatusOK)
	}
	inboxResponse.Body.Close()

	logoutResponse := doJSON(t, client, http.MethodPost, server.URL+"/auth/logout", nil)
	if logoutResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logoutResponse.StatusCode, http.StatusNoContent)
	}
	logoutResponse.Body.Close()

	assertErrorCode(t, doJSON(t, client, http.MethodGet, server.URL+"/inbox", nil), http.StatusUnauthorized, "AUTH_REQUIRED")
	assertErrorCode(t, doJSON(t, client, http.MethodPost, server.URL+"/auth/login", map[string]string{
		"email": "owner@example.com", "password": "wrong-password",
	}), http.StatusUnauthorized, "INVALID_CREDENTIALS")

	loginResponse := doJSON(t, client, http.MethodPost, server.URL+"/auth/login", map[string]string{
		"email": "OWNER@example.com", "password": "password123",
	})
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResponse.StatusCode, http.StatusOK)
	}
	loginResponse.Body.Close()
}

// TestWebsiteChannelLifecycle 验证网站渠道创建、修改、回收和恢复流程。
func TestWebsiteChannelLifecycle(t *testing.T) {
	application := newMemoryApplication()
	server := httptest.NewServer(newTestService(application))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	installResponse := doJSON(t, client, http.MethodPost, server.URL+"/install", map[string]string{
		"organizationName": "鹿行测试公司",
		"displayName":      "所有者",
		"email":            "owner@example.com",
		"password":         "password123",
	})
	installResponse.Body.Close()
	assertErrorCode(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website/not-a-uuid", nil), http.StatusNotFound, "CHANNEL_NOT_FOUND")

	createResponse := doJSON(t, client, http.MethodPost, server.URL+"/channels/website", map[string]string{
		"name":          "产品官网",
		"description":   "接收官网访客咨询",
		"defaultLocale": "zh-CN",
	})
	if createResponse.StatusCode != http.StatusCreated {
		createResponse.Body.Close()
		t.Fatalf("create status = %d, want %d", createResponse.StatusCode, http.StatusCreated)
	}
	var created servermodels.Channel
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		createResponse.Body.Close()
		t.Fatal(err)
	}
	createResponse.Body.Close()
	if created.Type != channelaction.TypeWebsite || created.CreatedByUserID != "user-1" {
		t.Fatalf("unexpected created channel: %#v", created)
	}

	updateResponse := doJSON(t, client, http.MethodPatch, server.URL+"/channels/website/"+created.ID, map[string]string{
		"name":          "帮助中心",
		"description":   "",
		"defaultLocale": "en-US",
	})
	if updateResponse.StatusCode != http.StatusOK {
		updateResponse.Body.Close()
		t.Fatalf("update status = %d, want %d", updateResponse.StatusCode, http.StatusOK)
	}
	var updated servermodels.Channel
	if err := json.NewDecoder(updateResponse.Body).Decode(&updated); err != nil {
		updateResponse.Body.Close()
		t.Fatal(err)
	}
	updateResponse.Body.Close()
	if updated.Name != "帮助中心" || updated.Description != nil || updated.DefaultLocale != "en-US" {
		t.Fatalf("unexpected updated channel: %#v", updated)
	}

	chatInterfaceResponse := doJSON(t, client, http.MethodPatch, server.URL+"/channels/website/"+created.ID+"/chat-interface", map[string]string{
		"title":           " 在线咨询 ",
		"subtitle":        " 通常会很快回复 ",
		"greetingMessage": " 你好，有什么可以帮你？ ",
		"themeColor":      "#16a34a",
	})
	if chatInterfaceResponse.StatusCode != http.StatusOK {
		chatInterfaceResponse.Body.Close()
		t.Fatalf("chat interface update status = %d, want %d", chatInterfaceResponse.StatusCode, http.StatusOK)
	}
	var chatInterface servermodels.WebsiteChannelSetting
	if err := json.NewDecoder(chatInterfaceResponse.Body).Decode(&chatInterface); err != nil {
		chatInterfaceResponse.Body.Close()
		t.Fatal(err)
	}
	chatInterfaceResponse.Body.Close()
	if chatInterface.ChatTitle != "在线咨询" || chatInterface.ChatSubtitle == nil || *chatInterface.ChatSubtitle != "通常会很快回复" || chatInterface.ThemeColor != "#16A34A" {
		t.Fatalf("unexpected chat interface: %#v", chatInterface)
	}

	detailResponse := doJSON(t, client, http.MethodGet, server.URL+"/channels/website/"+created.ID, nil)
	if detailResponse.StatusCode != http.StatusOK {
		detailResponse.Body.Close()
		t.Fatalf("detail status = %d, want %d", detailResponse.StatusCode, http.StatusOK)
	}
	var detail channelaction.WebsiteChannelDetail
	if err := json.NewDecoder(detailResponse.Body).Decode(&detail); err != nil {
		detailResponse.Body.Close()
		t.Fatal(err)
	}
	detailResponse.Body.Close()
	if detail.ChatInterface.ChatTitle != "在线咨询" || detail.ChatInterface.GreetingMessage == nil {
		t.Fatalf("unexpected website channel detail: %#v", detail)
	}

	deleteResponse := doJSON(t, client, http.MethodDelete, server.URL+"/channels/website/"+created.ID, nil)
	if deleteResponse.StatusCode != http.StatusNoContent {
		deleteResponse.Body.Close()
		t.Fatalf("delete status = %d, want %d", deleteResponse.StatusCode, http.StatusNoContent)
	}
	deleteResponse.Body.Close()
	assertChannelListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website", nil), 0)
	assertChannelListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website/trash", nil), 1)
	assertErrorCode(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website/"+created.ID, nil), http.StatusNotFound, "CHANNEL_NOT_FOUND")

	restoreResponse := doJSON(t, client, http.MethodPost, server.URL+"/channels/website/"+created.ID+"/restore", nil)
	if restoreResponse.StatusCode != http.StatusOK {
		restoreResponse.Body.Close()
		t.Fatalf("restore status = %d, want %d", restoreResponse.StatusCode, http.StatusOK)
	}
	restoreResponse.Body.Close()
	assertChannelListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website", nil), 1)
	assertChannelListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/channels/website/trash", nil), 0)
}

// TestContactLifecycle 验证联系人创建、修改、回收和恢复流程。
func TestContactLifecycle(t *testing.T) {
	application := newMemoryApplication()
	server := httptest.NewServer(newTestService(application))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	installResponse := doJSON(t, client, http.MethodPost, server.URL+"/install", map[string]string{
		"organizationName": "鹿行测试公司",
		"displayName":      "所有者",
		"email":            "owner@example.com",
		"password":         "password123",
	})
	installResponse.Body.Close()

	usersResponse := doJSON(t, client, http.MethodGet, server.URL+"/users", nil)
	if usersResponse.StatusCode != http.StatusOK {
		usersResponse.Body.Close()
		t.Fatalf("users status = %d, want %d", usersResponse.StatusCode, http.StatusOK)
	}
	usersResponse.Body.Close()

	createResponse := doJSON(t, client, http.MethodPost, server.URL+"/contacts", map[string]any{
		"displayName": "林晓",
		"channelId":   "00000000-0000-0000-0000-000000000001",
		"stage":       "lead",
		"notes":       "来自产品咨询",
		"methods": []map[string]any{
			{"type": "email", "value": "lin@example.com", "isPrimary": true},
		},
	})
	if createResponse.StatusCode != http.StatusCreated {
		createResponse.Body.Close()
		t.Fatalf("create contact status = %d, want %d", createResponse.StatusCode, http.StatusCreated)
	}
	var created contactaction.ContactDetail
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		createResponse.Body.Close()
		t.Fatal(err)
	}
	createResponse.Body.Close()
	if created.Contact.DisplayName == nil || *created.Contact.DisplayName != "林晓" || created.Contact.SourceChannelID != "00000000-0000-0000-0000-000000000001" || len(created.Methods) != 1 {
		t.Fatalf("unexpected created contact: %#v", created)
	}

	updateResponse := doJSON(t, client, http.MethodPatch, server.URL+"/contacts/"+created.Contact.ID, map[string]any{
		"displayName": "林晓（采购）",
		"channelId":   "00000000-0000-0000-0000-000000000001",
		"stage":       "customer",
		"notes":       "",
		"methods": []map[string]any{
			{"type": "phone", "value": "+8613800000000", "isPrimary": true},
		},
	})
	if updateResponse.StatusCode != http.StatusOK {
		updateResponse.Body.Close()
		t.Fatalf("update contact status = %d, want %d", updateResponse.StatusCode, http.StatusOK)
	}
	updateResponse.Body.Close()

	deleteResponse := doJSON(t, client, http.MethodDelete, server.URL+"/contacts/"+created.Contact.ID, nil)
	if deleteResponse.StatusCode != http.StatusNoContent {
		deleteResponse.Body.Close()
		t.Fatalf("delete contact status = %d, want %d", deleteResponse.StatusCode, http.StatusNoContent)
	}
	deleteResponse.Body.Close()
	assertContactListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/contacts", nil), 0)
	assertContactListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/contacts/trash", nil), 1)
	assertErrorCode(t, doJSON(t, client, http.MethodGet, server.URL+"/contacts/"+created.Contact.ID, nil), http.StatusNotFound, "CONTACT_NOT_FOUND")

	restoreResponse := doJSON(t, client, http.MethodPost, server.URL+"/contacts/"+created.Contact.ID+"/restore", nil)
	if restoreResponse.StatusCode != http.StatusOK {
		restoreResponse.Body.Close()
		t.Fatalf("restore contact status = %d, want %d", restoreResponse.StatusCode, http.StatusOK)
	}
	restoreResponse.Body.Close()
	assertContactListSize(t, doJSON(t, client, http.MethodGet, server.URL+"/contacts", nil), 1)
}

// TestErrorResponseUsesRequestedLanguage 验证 API 错误响应使用请求语言。
func TestErrorResponseUsesRequestedLanguage(t *testing.T) {
	server := httptest.NewServer(newTestService(newMemoryApplication()))
	defer server.Close()

	tests := []struct {
		language string
		message  string
	}{
		{language: "zh-CN", message: "Cervi 尚未完成初始化。"},
		{language: "en-US", message: "Cervi has not been initialized."},
	}

	for _, test := range tests {
		request, err := http.NewRequest(http.MethodGet, server.URL+"/inbox", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Accept-Language", test.language)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var payload errorBody
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			response.Body.Close()
			t.Fatal(err)
		}
		response.Body.Close()
		if payload.Error.Message != test.message {
			t.Fatalf("message = %q, want %q", payload.Error.Message, test.message)
		}
		if response.Header.Get("Content-Language") != test.language {
			t.Fatalf("Content-Language = %q, want %q", response.Header.Get("Content-Language"), test.language)
		}
	}
}

// doJSON 发送测试 JSON 请求并返回响应。
func doJSON(t *testing.T, client *http.Client, method, endpoint string, body any) *http.Response {
	t.Helper()
	var buffer bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buffer).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, endpoint, &buffer)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

// assertErrorCode 校验 API 错误的状态码和业务码。
func assertErrorCode(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != status {
		t.Fatalf("status = %d, want %d", response.StatusCode, status)
	}
	var payload errorBody
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != code {
		t.Fatalf("error code = %q, want %q", payload.Error.Code, code)
	}
}

// assertChannelListSize 校验网站渠道列表长度。
func assertChannelListSize(t *testing.T, response *http.Response, size int) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var payload struct {
		Channels []servermodels.Channel `json:"channels"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Channels) != size {
		t.Fatalf("channel count = %d, want %d", len(payload.Channels), size)
	}
}

// assertContactListSize 校验联系人列表长度。
func assertContactListSize(t *testing.T, response *http.Response, size int) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("contact list status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var payload contactaction.ListOutput
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Contacts) != size || payload.Page.Total != size {
		t.Fatalf("contact count = %d/%d, want %d", len(payload.Contacts), payload.Page.Total, size)
	}
}

// TestSessionCookieUsesSecureFlagBehindHTTPSProxy 验证 HTTPS 代理下的安全 Cookie 标记。
func TestSessionCookieUsesSecureFlagBehindHTTPSProxy(t *testing.T) {
	context, _ := ginTestContext(t)
	request := httptest.NewRequest(http.MethodPost, "/install", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	context.Request = request

	setSessionCookie(context, "token", time.Now().Add(time.Hour))
	response := context.Writer.Header().Values("Set-Cookie")
	if len(response) != 1 || !containsCookieAttribute(response[0], "Secure") || !containsCookieAttribute(response[0], "HttpOnly") {
		t.Fatalf("Set-Cookie = %v, want Secure and HttpOnly", response)
	}
}

// TestLogoutPreservesCookieWhenSessionDeletionFails 验证会话删除失败时保留登录 Cookie。
func TestLogoutPreservesCookieWhenSessionDeletionFails(t *testing.T) {
	application := newMemoryApplication()
	server := httptest.NewServer(newTestService(application))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	installResponse := doJSON(t, client, http.MethodPost, server.URL+"/install", map[string]string{
		"organizationName": "鹿行测试公司",
		"displayName":      "所有者",
		"email":            "owner@example.com",
		"password":         "password123",
	})
	installResponse.Body.Close()

	application.deleteErr = errors.New("database unavailable")
	assertErrorCode(t, doJSON(t, client, http.MethodPost, server.URL+"/auth/logout", nil), http.StatusInternalServerError, "LOGOUT_FAILED")

	inboxResponse := doJSON(t, client, http.MethodGet, server.URL+"/inbox", nil)
	defer inboxResponse.Body.Close()
	if inboxResponse.StatusCode != http.StatusOK {
		t.Fatalf("inbox status after failed logout = %d, want %d", inboxResponse.StatusCode, http.StatusOK)
	}
}

// ginTestContext 创建带响应记录器的 Gin 测试上下文。
func ginTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	return context, recorder
}

// containsCookieAttribute 判断 Cookie 是否包含指定属性。
func containsCookieAttribute(value, attribute string) bool {
	return strings.Contains(value, "; "+attribute)
}
