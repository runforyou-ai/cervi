//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

var (
	_ Backend            = (*DirectBackend)(nil)
	_ WorkspaceInstaller = (*DirectBackend)(nil)
)

// DirectBackend 在服务端进程内直接调用 Action 和 Query。
type DirectBackend struct {
	installWorkspace                  *installationaction.InstallWorkspaceAction
	login                             *authaction.LoginAction
	logout                            *authaction.LogoutAction
	resolveIdentity                   *authaction.ResolveIdentityQuery
	installation                      *installationaction.StatusQuery
	loadInbox                         *inboxaction.LoadInboxQuery
	listWebsiteChannels               *channelaction.ListWebsiteChannelsQuery
	getWebsiteChannel                 *channelaction.GetWebsiteChannelQuery
	createWebsiteChannel              *channelaction.CreateWebsiteChannelAction
	updateWebsiteChannel              *channelaction.UpdateWebsiteChannelAction
	updateWebsiteChannelChatInterface *channelaction.UpdateWebsiteChannelChatInterfaceAction
	deleteWebsiteChannel              *channelaction.DeleteWebsiteChannelAction
	restoreWebsiteChannel             *channelaction.RestoreWebsiteChannelAction
	listChannels                      *channelaction.ListChannelsQuery
	listUsers                         *useraction.ListUsersQuery
	getUser                           *useraction.GetUserQuery
	listContacts                      *contactaction.ListContactsQuery
	getContact                        *contactaction.GetContactQuery
	createContact                     *contactaction.CreateContactAction
	updateContact                     *contactaction.UpdateContactAction
	deleteContact                     *contactaction.DeleteContactAction
	restoreContact                    *contactaction.RestoreContactAction
	getS3Setting                      *settingaction.GetS3SettingQuery
	saveS3Setting                     *settingaction.SaveS3SettingAction
	testS3Setting                     *settingaction.TestS3SettingAction
}

// NewDirectBackend 创建直接访问服务端存储的应用后端。
func NewDirectBackend(db *bun.DB) *DirectBackend {
	return &DirectBackend{
		installWorkspace:                  installationaction.NewInstallWorkspaceAction(db),
		login:                             authaction.NewLoginAction(db),
		logout:                            authaction.NewLogoutAction(db),
		resolveIdentity:                   authaction.NewResolveIdentityQuery(db),
		installation:                      installationaction.NewStatusQuery(db),
		loadInbox:                         inboxaction.NewLoadInboxQuery(),
		listWebsiteChannels:               channelaction.NewListWebsiteChannelsQuery(db),
		getWebsiteChannel:                 channelaction.NewGetWebsiteChannelQuery(db),
		createWebsiteChannel:              channelaction.NewCreateWebsiteChannelAction(db),
		updateWebsiteChannel:              channelaction.NewUpdateWebsiteChannelAction(db),
		updateWebsiteChannelChatInterface: channelaction.NewUpdateWebsiteChannelChatInterfaceAction(db),
		deleteWebsiteChannel:              channelaction.NewDeleteWebsiteChannelAction(db),
		restoreWebsiteChannel:             channelaction.NewRestoreWebsiteChannelAction(db),
		listChannels:                      channelaction.NewListChannelsQuery(db),
		listUsers:                         useraction.NewListUsersQuery(db),
		getUser:                           useraction.NewGetUserQuery(db),
		listContacts:                      contactaction.NewListContactsQuery(db),
		getContact:                        contactaction.NewGetContactQuery(db),
		createContact:                     contactaction.NewCreateContactAction(db),
		updateContact:                     contactaction.NewUpdateContactAction(db),
		deleteContact:                     contactaction.NewDeleteContactAction(db),
		restoreContact:                    contactaction.NewRestoreContactAction(db),
		getS3Setting:                      settingaction.NewGetS3SettingQuery(db),
		saveS3Setting:                     settingaction.NewSaveS3SettingAction(db),
		testS3Setting:                     settingaction.NewTestS3SettingAction(),
	}
}

// InstallationStatus 返回服务端是否已完成初始化。
func (b *DirectBackend) InstallationStatus(ctx context.Context, meta RequestMeta) (bool, error) {
	installed, err := b.installation.Execute(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		slog.Warn("读取初始化状态失败", "error", err)
		return false, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorInstallationStatusReadFailed, nil)
	}
	return installed, nil
}

// InstallWorkspace 创建企业所有者并返回登录令牌。
func (b *DirectBackend) InstallWorkspace(ctx context.Context, meta RequestMeta, input InstallWorkspaceInput) (Auth, error) {
	installed, err := b.InstallationStatus(ctx, meta)
	if err != nil {
		return Auth{}, err
	}
	if installed {
		return Auth{}, localizedError(meta, http.StatusConflict, "ALREADY_INITIALIZED", cervii18n.ErrorAlreadyInitialized, nil)
	}
	output, err := b.installWorkspace.Execute(ctx, installationaction.InstallWorkspaceInput{
		OrganizationName: input.OrganizationName,
		DisplayName:      input.DisplayName,
		Email:            input.Email,
		Password:         input.Password,
	})
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return Auth{}, localizedError(meta, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, installationFieldKeys(validationError.Fields))
	}
	if errors.Is(err, installationaction.ErrAlreadyInstalled) {
		return Auth{}, localizedError(meta, http.StatusConflict, "ALREADY_INITIALIZED", cervii18n.ErrorAlreadyInitialized, nil)
	}
	if err != nil {
		if ctx.Err() != nil {
			return Auth{}, ctx.Err()
		}
		slog.Warn("初始化企业失败", "error", err)
		return Auth{}, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorInstallationFailed, nil)
	}
	slog.Info("企业初始化完成", "organization_id", output.Identity.Organization.ID, "owner_id", output.Identity.User.ID)
	return Auth{Identity: identityFromModel(output.Identity), Token: output.Token, ExpiresAt: output.ExpiresAt}, nil
}

// Login 校验账号密码并返回登录令牌。
func (b *DirectBackend) Login(ctx context.Context, meta RequestMeta, input LoginInput) (Auth, error) {
	if err := b.requireInitialized(ctx, meta); err != nil {
		return Auth{}, err
	}
	output, err := b.login.Execute(ctx, authaction.LoginInput{Email: input.Email, Password: input.Password})
	if errors.Is(err, authaction.ErrInvalidCredentials) {
		return Auth{}, localizedError(meta, http.StatusUnauthorized, "INVALID_CREDENTIALS", cervii18n.ErrorInvalidCredentials, nil)
	}
	if err != nil {
		if ctx.Err() != nil {
			return Auth{}, ctx.Err()
		}
		slog.Warn("用户登录失败", "error", err)
		return Auth{}, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorLoginFailed, nil)
	}
	slog.Info("用户登录成功", "organization_id", output.Identity.Organization.ID, "user_id", output.Identity.User.ID)
	return Auth{Identity: identityFromModel(output.Identity), Token: output.Token, ExpiresAt: output.ExpiresAt}, nil
}

// Logout 删除当前登录令牌。
func (b *DirectBackend) Logout(ctx context.Context, meta RequestMeta) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.logout.Execute(ctx, meta.Token); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("删除登录令牌失败", "user_id", identity.User.ID, "error", err)
		return localizedError(meta, http.StatusInternalServerError, "LOGOUT_FAILED", cervii18n.ErrorLogoutFailed, nil)
	}
	slog.Info("用户退出登录", "organization_id", identity.Organization.ID, "user_id", identity.User.ID)
	return nil
}

// LoadIdentity 返回令牌对应的当前身份。
func (b *DirectBackend) LoadIdentity(ctx context.Context, meta RequestMeta) (Identity, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Identity{}, err
	}
	return identityFromModel(identity), nil
}

// LoadInbox 返回当前身份可访问的统一收件箱。
func (b *DirectBackend) LoadInbox(ctx context.Context, meta RequestMeta) (Inbox, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Inbox{}, err
	}
	output := b.loadInbox.Execute(ctx, identity)
	return Inbox{
		Organization:  organizationFromModel(output.Organization),
		User:          userFromModel(output.User),
		Conversations: []Conversation{},
	}, nil
}

// ListWebsiteChannels 返回当前企业的网站渠道。
func (b *DirectBackend) ListWebsiteChannels(ctx context.Context, meta RequestMeta, deleted bool) (WebsiteChannelList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelList{}, err
	}
	channels, err := b.listWebsiteChannels.Execute(ctx, identity, deleted)
	if err != nil {
		if ctx.Err() != nil {
			return WebsiteChannelList{}, ctx.Err()
		}
		slog.Warn("读取网站渠道列表失败", "organization_id", identity.Organization.ID, "deleted", deleted, "error", err)
		return WebsiteChannelList{}, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorChannelListFailed, nil)
	}
	result := make([]WebsiteChannelSummary, 0, len(channels))
	for index := range channels {
		result = append(result, websiteChannelFromModel(&channels[index]))
	}
	return WebsiteChannelList{Channels: result}, nil
}

// GetWebsiteChannel 返回网站渠道详情。
func (b *DirectBackend) GetWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) (WebsiteChannel, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannel{}, err
	}
	detail, err := b.getWebsiteChannel.Execute(ctx, identity, channelID)
	if err != nil {
		return WebsiteChannel{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelReadFailed)
	}
	return WebsiteChannel{
		WebsiteChannelSummary: websiteChannelFromModel(detail.Channel),
		ChatInterface:         websiteChannelSettingFromModel(&detail.ChatInterface),
	}, nil
}

// CreateWebsiteChannel 创建网站渠道。
func (b *DirectBackend) CreateWebsiteChannel(ctx context.Context, meta RequestMeta, input WebsiteChannelInput) (WebsiteChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelSummary{}, err
	}
	channel, err := b.createWebsiteChannel.Execute(ctx, identity, channelInput(input))
	if err != nil {
		return WebsiteChannelSummary{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelCreateFailed)
	}
	slog.Info("网站渠道创建成功", "organization_id", identity.Organization.ID, "channel_id", channel.ID)
	return websiteChannelFromModel(channel), nil
}

// UpdateWebsiteChannel 修改网站渠道。
func (b *DirectBackend) UpdateWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string, input WebsiteChannelInput) (WebsiteChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelSummary{}, err
	}
	channel, err := b.updateWebsiteChannel.Execute(ctx, identity, channelID, channelInput(input))
	if err != nil {
		return WebsiteChannelSummary{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelUpdateFailed)
	}
	slog.Info("网站渠道更新成功", "organization_id", identity.Organization.ID, "channel_id", channel.ID)
	return websiteChannelFromModel(channel), nil
}

// UpdateWebsiteChannelChatInterface 修改网站渠道聊天界面。
func (b *DirectBackend) UpdateWebsiteChannelChatInterface(ctx context.Context, meta RequestMeta, channelID string, input WebsiteChannelChatInterfaceInput) (WebsiteChannelChatInterface, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelChatInterface{}, err
	}
	setting, err := b.updateWebsiteChannelChatInterface.Execute(ctx, identity, channelID, channelaction.WebsiteChannelChatInterfaceInput{
		Title: input.Title, Subtitle: input.Subtitle, GreetingMessage: input.GreetingMessage, ThemeColor: input.ThemeColor,
	})
	if err != nil {
		return WebsiteChannelChatInterface{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelChatInterfaceUpdateFailed)
	}
	slog.Info("网站渠道聊天界面更新成功", "organization_id", identity.Organization.ID, "channel_id", channelID)
	return websiteChannelSettingFromModel(setting), nil
}

// DeleteWebsiteChannel 将网站渠道移入回收站。
func (b *DirectBackend) DeleteWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteWebsiteChannel.Execute(ctx, identity, channelID); err != nil {
		return b.channelError(ctx, meta, err, cervii18n.ErrorChannelDeleteFailed)
	}
	slog.Info("网站渠道移入回收站", "organization_id", identity.Organization.ID, "channel_id", channelID)
	return nil
}

// RestoreWebsiteChannel 恢复网站渠道。
func (b *DirectBackend) RestoreWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) (WebsiteChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelSummary{}, err
	}
	channel, err := b.restoreWebsiteChannel.Execute(ctx, identity, channelID)
	if err != nil {
		return WebsiteChannelSummary{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelRestoreFailed)
	}
	slog.Info("网站渠道恢复成功", "organization_id", identity.Organization.ID, "channel_id", channel.ID)
	return websiteChannelFromModel(channel), nil
}

// ListChannels 返回当前企业的渠道选择项。
func (b *DirectBackend) ListChannels(ctx context.Context, meta RequestMeta) (ChannelList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ChannelList{}, err
	}
	channels, err := b.listChannels.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return ChannelList{}, ctx.Err()
		}
		slog.Warn("读取渠道列表失败", "organization_id", identity.Organization.ID, "error", err)
		return ChannelList{}, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorChannelSummaryListFailed, nil)
	}
	result := make([]ChannelSummary, 0, len(channels))
	for _, channel := range channels {
		result = append(result, ChannelSummary{ID: channel.ID, Type: ChannelType(channel.Type), Name: channel.Name})
	}
	return ChannelList{Channels: result}, nil
}

// ListUsers 返回企业成员列表。
func (b *DirectBackend) ListUsers(ctx context.Context, meta RequestMeta, input UserListInput) (UserList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return UserList{}, err
	}
	output, err := b.listUsers.Execute(ctx, identity, useraction.ListInput{
		Query: input.Query, Status: optionalDomain[UserStatus, domain.UserStatus](input.Status), Role: optionalDomain[UserRole, domain.UserRole](input.Role), Page: input.Page, PageSize: input.PageSize,
	})
	if errors.Is(err, useraction.ErrQueryInvalid) {
		return UserList{}, localizedError(meta, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, nil)
	}
	if err != nil {
		if ctx.Err() != nil {
			return UserList{}, ctx.Err()
		}
		slog.Warn("读取企业成员列表失败", "organization_id", identity.Organization.ID, "error", err)
		return UserList{}, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorUserListFailed, nil)
	}
	users := make([]DirectoryUser, 0, len(output.Users))
	for _, user := range output.Users {
		users = append(users, DirectoryUser{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Role: UserRole(user.Role), Status: UserStatus(user.Status), CreatedAt: user.CreatedAt})
	}
	return UserList{Users: users, Page: PageInfo{Number: output.Page.Number, Size: output.Page.Size, Total: output.Page.Total}}, nil
}

// GetUser 返回企业成员详情。
func (b *DirectBackend) GetUser(ctx context.Context, meta RequestMeta, userID string) (DirectoryUser, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return DirectoryUser{}, err
	}
	user, err := b.getUser.Execute(ctx, identity, userID)
	if errors.Is(err, useraction.ErrNotFound) {
		return DirectoryUser{}, localizedError(meta, http.StatusNotFound, "USER_NOT_FOUND", cervii18n.ErrorUserNotFound, nil)
	}
	if err != nil {
		if ctx.Err() != nil {
			return DirectoryUser{}, ctx.Err()
		}
		slog.Warn("读取企业成员失败", "organization_id", identity.Organization.ID, "user_id", userID, "error", err)
		return DirectoryUser{}, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorUserReadFailed, nil)
	}
	return DirectoryUser{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Role: UserRole(user.Role), Status: UserStatus(user.Status), CreatedAt: user.CreatedAt}, nil
}

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

// GetS3Setting 返回当前企业的对象存储设置。
func (b *DirectBackend) GetS3Setting(ctx context.Context, meta RequestMeta) (S3Setting, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return S3Setting{}, err
	}
	setting, err := b.getS3Setting.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return S3Setting{}, ctx.Err()
		}
		slog.Warn("读取对象存储设置失败", "organization_id", identity.Organization.ID, "error", err)
		return S3Setting{}, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorS3SettingReadFailed, nil)
	}
	return s3SettingFromAction(setting), nil
}

// SaveS3Setting 保存当前企业的对象存储设置。
func (b *DirectBackend) SaveS3Setting(ctx context.Context, meta RequestMeta, input S3Setting) (S3Setting, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return S3Setting{}, err
	}
	setting, err := b.saveS3Setting.Execute(ctx, identity, s3SettingToAction(input))
	if err != nil {
		return S3Setting{}, b.s3SettingError(ctx, meta, err, cervii18n.ErrorS3SettingSaveFailed)
	}
	slog.Info("对象存储设置保存成功", "organization_id", identity.Organization.ID, "provider", setting.Provider, "enabled", setting.Enabled)
	return s3SettingFromAction(setting), nil
}

// TestS3Setting 测试对象存储连接。
func (b *DirectBackend) TestS3Setting(ctx context.Context, meta RequestMeta, input S3Setting) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.testS3Setting.Execute(ctx, s3SettingToAction(input)); err != nil {
		return b.s3SettingError(ctx, meta, err, cervii18n.ErrorS3ConnectionTestFailed)
	}
	slog.Info("对象存储连接测试成功", "organization_id", identity.Organization.ID, "provider", input.Provider)
	return nil
}

func (b *DirectBackend) requireInitialized(ctx context.Context, meta RequestMeta) error {
	installed, err := b.InstallationStatus(ctx, meta)
	if err != nil {
		return err
	}
	if !installed {
		return localizedError(meta, http.StatusConflict, "INSTALLATION_REQUIRED", cervii18n.ErrorInstallationRequired, nil)
	}
	return nil
}

func (b *DirectBackend) authenticate(ctx context.Context, meta RequestMeta) (*servermodels.Identity, error) {
	if err := b.requireInitialized(ctx, meta); err != nil {
		return nil, err
	}
	if meta.Token == "" {
		return nil, localizedError(meta, http.StatusUnauthorized, "AUTH_REQUIRED", cervii18n.ErrorAuthenticationRequired, nil)
	}
	identity, err := b.resolveIdentity.Execute(ctx, meta.Token)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		slog.Warn("读取登录令牌失败", "error", err)
		return nil, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorAuthenticationStatusFailed, nil)
	}
	if identity == nil {
		return nil, localizedError(meta, http.StatusUnauthorized, "AUTH_REQUIRED", cervii18n.ErrorAuthenticationRequired, nil)
	}
	return identity, nil
}

func optionalDomain[T ~string, D ~string](value *T) D {
	if value == nil {
		return ""
	}
	return D(*value)
}
