package appservice

import "context"

// Backend 定义各运行平台都需要实现的业务调用。
type Backend interface {
	InstallationStatus(context.Context, RequestMeta) (InstallationStatus, error)
	Login(context.Context, RequestMeta, LoginInput) (Auth, error)
	Logout(context.Context, RequestMeta) error
	LoadIdentity(context.Context, RequestMeta) (Identity, error)
	UpdateProfile(context.Context, RequestMeta, ProfileInput) (User, error)
	ChangePassword(context.Context, RequestMeta, ChangePasswordInput) error
	UpdateUserPreferences(context.Context, RequestMeta, UserPreferencesInput) (User, error)
	UpdateUserWorkStatus(context.Context, RequestMeta, UserWorkStatusInput) (User, error)
	LoadInbox(context.Context, RequestMeta) (Inbox, error)
	ListWebsiteChannels(context.Context, RequestMeta, bool) (WebsiteChannelList, error)
	GetWebsiteChannel(context.Context, RequestMeta, string) (WebsiteChannel, error)
	CreateWebsiteChannel(context.Context, RequestMeta, WebsiteChannelInput) (WebsiteChannelSummary, error)
	UpdateWebsiteChannel(context.Context, RequestMeta, string, WebsiteChannelInput) (WebsiteChannelSummary, error)
	UpdateWebsiteChannelChatInterface(context.Context, RequestMeta, string, WebsiteChannelChatInterfaceInput) (WebsiteChannelChatInterface, error)
	DeleteWebsiteChannel(context.Context, RequestMeta, string) error
	RestoreWebsiteChannel(context.Context, RequestMeta, string) (WebsiteChannelSummary, error)
	ListChannels(context.Context, RequestMeta) (ChannelList, error)
	ListUsers(context.Context, RequestMeta, UserListInput) (UserList, error)
	GetUser(context.Context, RequestMeta, string) (DirectoryUser, error)
	ListContacts(context.Context, RequestMeta, ContactListInput) (ContactList, error)
	GetContact(context.Context, RequestMeta, string) (Contact, error)
	CreateContact(context.Context, RequestMeta, ContactInput) (Contact, error)
	UpdateContact(context.Context, RequestMeta, string, ContactInput) (Contact, error)
	DeleteContact(context.Context, RequestMeta, string) error
	RestoreContact(context.Context, RequestMeta, string) (Contact, error)
	UpdateOrganization(context.Context, RequestMeta, OrganizationInput) (Organization, error)
	GetS3Setting(context.Context, RequestMeta) (S3Setting, error)
	SaveS3Setting(context.Context, RequestMeta, S3Setting) (S3Setting, error)
	TestS3Setting(context.Context, RequestMeta, S3Setting) error
}

// WorkspaceInstaller 由服务端 Backend 实现，用于企业初始化。
type WorkspaceInstaller interface {
	InstallWorkspace(context.Context, RequestMeta, InstallWorkspaceInput) (Auth, error)
}

// ServerConnector 由原生端 Backend 实现，用于企业服务器地址。
type ServerConnector interface {
	ServerURL(context.Context, RequestMeta) (string, error)
	ProbeServer(context.Context, RequestMeta, string) (InstallationStatus, error)
	ConnectServer(context.Context, RequestMeta, string) (bool, error)
}

// Service 将跨平台业务调用转发给当前运行平台的 Backend。
type Service struct {
	backend Backend
}

// New 创建跨平台应用服务。
func New(backend Backend) *Service {
	return &Service{backend: backend}
}

// InstallationStatus 返回服务端初始化状态和公开企业名称。
func (s *Service) InstallationStatus(ctx context.Context, meta RequestMeta) (InstallationStatus, error) {
	return s.backend.InstallationStatus(ctx, meta)
}

// InstallWorkspace 创建企业管理员并返回登录令牌。
func (s *Service) InstallWorkspace(ctx context.Context, meta RequestMeta, input InstallWorkspaceInput) (Auth, error) {
	installer, ok := s.backend.(WorkspaceInstaller)
	if !ok {
		return Auth{}, methodNotAllowedError(meta, "InstallWorkspace")
	}
	return installer.InstallWorkspace(ctx, meta, input)
}

// Login 校验账号密码并返回登录令牌。
func (s *Service) Login(ctx context.Context, meta RequestMeta, input LoginInput) (Auth, error) {
	return s.backend.Login(ctx, meta, input)
}

// Logout 删除当前登录令牌。
func (s *Service) Logout(ctx context.Context, meta RequestMeta) error {
	return s.backend.Logout(ctx, meta)
}

// LoadIdentity 返回当前登录身份。
func (s *Service) LoadIdentity(ctx context.Context, meta RequestMeta) (Identity, error) {
	return s.backend.LoadIdentity(ctx, meta)
}

// UpdateProfile 修改当前用户的姓名和邮箱。
func (s *Service) UpdateProfile(ctx context.Context, meta RequestMeta, input ProfileInput) (User, error) {
	return s.backend.UpdateProfile(ctx, meta, input)
}

// ChangePassword 核验当前密码并保存新密码。
func (s *Service) ChangePassword(ctx context.Context, meta RequestMeta, input ChangePasswordInput) error {
	return s.backend.ChangePassword(ctx, meta, input)
}

// UpdateUserPreferences 保存当前用户的语言和时区设置。
func (s *Service) UpdateUserPreferences(ctx context.Context, meta RequestMeta, input UserPreferencesInput) (User, error) {
	return s.backend.UpdateUserPreferences(ctx, meta, input)
}

// UpdateUserWorkStatus 保存当前用户主动设置的工作状态。
func (s *Service) UpdateUserWorkStatus(ctx context.Context, meta RequestMeta, input UserWorkStatusInput) (User, error) {
	return s.backend.UpdateUserWorkStatus(ctx, meta, input)
}

// LoadInbox 返回当前用户的统一收件箱。
func (s *Service) LoadInbox(ctx context.Context, meta RequestMeta) (Inbox, error) {
	return s.backend.LoadInbox(ctx, meta)
}

// ListWebsiteChannels 返回网站渠道列表。
func (s *Service) ListWebsiteChannels(ctx context.Context, meta RequestMeta, deleted bool) (WebsiteChannelList, error) {
	return s.backend.ListWebsiteChannels(ctx, meta, deleted)
}

// GetWebsiteChannel 返回网站渠道详情。
func (s *Service) GetWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) (WebsiteChannel, error) {
	return s.backend.GetWebsiteChannel(ctx, meta, channelID)
}

// CreateWebsiteChannel 创建网站渠道。
func (s *Service) CreateWebsiteChannel(ctx context.Context, meta RequestMeta, input WebsiteChannelInput) (WebsiteChannelSummary, error) {
	return s.backend.CreateWebsiteChannel(ctx, meta, input)
}

// UpdateWebsiteChannel 修改网站渠道。
func (s *Service) UpdateWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string, input WebsiteChannelInput) (WebsiteChannelSummary, error) {
	return s.backend.UpdateWebsiteChannel(ctx, meta, channelID, input)
}

// UpdateWebsiteChannelChatInterface 修改网站渠道聊天界面。
func (s *Service) UpdateWebsiteChannelChatInterface(ctx context.Context, meta RequestMeta, channelID string, input WebsiteChannelChatInterfaceInput) (WebsiteChannelChatInterface, error) {
	return s.backend.UpdateWebsiteChannelChatInterface(ctx, meta, channelID, input)
}

// DeleteWebsiteChannel 将网站渠道移入回收站。
func (s *Service) DeleteWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) error {
	return s.backend.DeleteWebsiteChannel(ctx, meta, channelID)
}

// RestoreWebsiteChannel 恢复网站渠道。
func (s *Service) RestoreWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) (WebsiteChannelSummary, error) {
	return s.backend.RestoreWebsiteChannel(ctx, meta, channelID)
}

// ListChannels 返回当前企业的渠道选择项。
func (s *Service) ListChannels(ctx context.Context, meta RequestMeta) (ChannelList, error) {
	return s.backend.ListChannels(ctx, meta)
}

// ListUsers 返回企业成员列表。
func (s *Service) ListUsers(ctx context.Context, meta RequestMeta, input UserListInput) (UserList, error) {
	return s.backend.ListUsers(ctx, meta, input)
}

// GetUser 返回企业成员详情。
func (s *Service) GetUser(ctx context.Context, meta RequestMeta, userID string) (DirectoryUser, error) {
	return s.backend.GetUser(ctx, meta, userID)
}

// ListContacts 返回联系人列表。
func (s *Service) ListContacts(ctx context.Context, meta RequestMeta, input ContactListInput) (ContactList, error) {
	return s.backend.ListContacts(ctx, meta, input)
}

// GetContact 返回联系人详情。
func (s *Service) GetContact(ctx context.Context, meta RequestMeta, contactID string) (Contact, error) {
	return s.backend.GetContact(ctx, meta, contactID)
}

// CreateContact 创建联系人。
func (s *Service) CreateContact(ctx context.Context, meta RequestMeta, input ContactInput) (Contact, error) {
	return s.backend.CreateContact(ctx, meta, input)
}

// UpdateContact 修改联系人。
func (s *Service) UpdateContact(ctx context.Context, meta RequestMeta, contactID string, input ContactInput) (Contact, error) {
	return s.backend.UpdateContact(ctx, meta, contactID, input)
}

// DeleteContact 将联系人移入回收站。
func (s *Service) DeleteContact(ctx context.Context, meta RequestMeta, contactID string) error {
	return s.backend.DeleteContact(ctx, meta, contactID)
}

// RestoreContact 恢复联系人。
func (s *Service) RestoreContact(ctx context.Context, meta RequestMeta, contactID string) (Contact, error) {
	return s.backend.RestoreContact(ctx, meta, contactID)
}

// UpdateOrganization 修改当前企业名称。
func (s *Service) UpdateOrganization(ctx context.Context, meta RequestMeta, input OrganizationInput) (Organization, error) {
	return s.backend.UpdateOrganization(ctx, meta, input)
}

// GetS3Setting 返回当前企业的对象存储设置。
func (s *Service) GetS3Setting(ctx context.Context, meta RequestMeta) (S3Setting, error) {
	return s.backend.GetS3Setting(ctx, meta)
}

// SaveS3Setting 保存当前企业的对象存储设置。
func (s *Service) SaveS3Setting(ctx context.Context, meta RequestMeta, input S3Setting) (S3Setting, error) {
	return s.backend.SaveS3Setting(ctx, meta, input)
}

// TestS3Setting 测试对象存储连接。
func (s *Service) TestS3Setting(ctx context.Context, meta RequestMeta, input S3Setting) error {
	return s.backend.TestS3Setting(ctx, meta, input)
}

// ServerURL 返回原生端当前配置的企业服务器地址。
func (s *Service) ServerURL(ctx context.Context, meta RequestMeta) (string, error) {
	connector, ok := s.backend.(ServerConnector)
	if !ok {
		return "", methodNotAllowedError(meta, "ServerURL")
	}
	return connector.ServerURL(ctx, meta)
}

// ProbeServer 检测企业服务器并返回公开企业名称，不保存地址。
func (s *Service) ProbeServer(ctx context.Context, meta RequestMeta, serverURL string) (InstallationStatus, error) {
	connector, ok := s.backend.(ServerConnector)
	if !ok {
		return InstallationStatus{}, methodNotAllowedError(meta, "ProbeServer")
	}
	return connector.ProbeServer(ctx, meta, serverURL)
}

// ConnectServer 验证并保存原生端企业服务器地址，并返回地址是否变化。
func (s *Service) ConnectServer(ctx context.Context, meta RequestMeta, serverURL string) (bool, error) {
	connector, ok := s.backend.(ServerConnector)
	if !ok {
		return false, methodNotAllowedError(meta, "ConnectServer")
	}
	return connector.ConnectServer(ctx, meta, serverURL)
}
