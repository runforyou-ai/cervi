// Package appservice 定义跨平台应用服务及其传输契约。
package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// Locale 表示应用支持的本地化语言。
type Locale string

const (
	LocaleChineseSimplified   Locale = Locale(domain.LocaleChineseSimplified)
	LocaleEnglishUnitedStates Locale = Locale(domain.LocaleEnglishUnitedStates)
)

// SessionState 表示会话入口。
type SessionState string

const (
	SessionStateReady   SessionState = "ready"
	SessionStateLogin   SessionState = "login"
	SessionStateSetup   SessionState = "setup"
	SessionStateConnect SessionState = "connect"
)

// ErrorKind 表示业务失败种类。
type ErrorKind string

const (
	ErrorKindInvalid     ErrorKind = "invalid"
	ErrorKindNotFound    ErrorKind = "not_found"
	ErrorKindUnavailable ErrorKind = "unavailable"
	ErrorKindFailed      ErrorKind = "failed"
)

// Session 表示当前会话。
type Session struct {
	State            SessionState `json:"state"`
	Identity         *Identity    `json:"identity,omitempty"`
	OrganizationName string       `json:"organizationName,omitempty"`
}

// UserRole 表示企业成员角色。
type UserRole string

const (
	UserRoleAdmin  UserRole = UserRole(domain.UserRoleAdmin)
	UserRoleMember UserRole = UserRole(domain.UserRoleMember)
)

// UserStatus 表示企业成员状态。
type UserStatus string

const (
	UserStatusActive   UserStatus = UserStatus(domain.UserStatusActive)
	UserStatusInactive UserStatus = UserStatus(domain.UserStatusInactive)
)

// MemberIdentityType 表示可以加入团队的一等身份类型。
type MemberIdentityType string

const (
	MemberIdentityTypeUser  MemberIdentityType = MemberIdentityType(domain.MemberIdentityTypeUser)
	MemberIdentityTypeAgent MemberIdentityType = MemberIdentityType(domain.MemberIdentityTypeAgent)
)

// WorkStatus 表示成员主动设置的工作状态。
type WorkStatus string

const (
	WorkStatusWorking WorkStatus = WorkStatus(domain.WorkStatusWorking)
	WorkStatusAway    WorkStatus = WorkStatus(domain.WorkStatusAway)
	WorkStatusOffDuty WorkStatus = WorkStatus(domain.WorkStatusOffDuty)
)

// ChannelType 表示渠道类型。
type ChannelType string

const (
	ChannelTypeWebsite ChannelType = ChannelType(domain.ChannelTypeWebsite)
)

// MessageAuthor 表示消息发送方。
type MessageAuthor string

const (
	MessageAuthorVisitor MessageAuthor = MessageAuthor(domain.MessageAuthorVisitor)
	MessageAuthorAgent   MessageAuthor = MessageAuthor(domain.MessageAuthorAgent)
)

// ContactStage 表示联系人阶段。
type ContactStage string

const (
	ContactStageVisitor  ContactStage = ContactStage(domain.ContactStageVisitor)
	ContactStageLead     ContactStage = ContactStage(domain.ContactStageLead)
	ContactStageCustomer ContactStage = ContactStage(domain.ContactStageCustomer)
)

// ContactMethodType 表示联系人联系方式类型。
type ContactMethodType string

const (
	ContactMethodTypeEmail ContactMethodType = ContactMethodType(domain.ContactMethodTypeEmail)
	ContactMethodTypePhone ContactMethodType = ContactMethodType(domain.ContactMethodTypePhone)
)

// ContactSort 表示联系人列表排序方式。
type ContactSort string

const (
	ContactSortUpdatedAtDescending  ContactSort = ContactSort(domain.ContactSortUpdatedAtDescending)
	ContactSortCreatedAtDescending  ContactSort = ContactSort(domain.ContactSortCreatedAtDescending)
	ContactSortDisplayNameAscending ContactSort = ContactSort(domain.ContactSortDisplayNameAscending)
)

// StorageProvider 表示 S3 兼容对象存储提供商。
type StorageProvider string

const (
	StorageProviderGeneric StorageProvider = StorageProvider(domain.StorageProviderGeneric)
	StorageProviderAWS     StorageProvider = StorageProvider(domain.StorageProviderAWS)
	StorageProviderR2      StorageProvider = StorageProvider(domain.StorageProviderR2)
	StorageProviderAliyun  StorageProvider = StorageProvider(domain.StorageProviderAliyun)
	StorageProviderTencent StorageProvider = StorageProvider(domain.StorageProviderTencent)
	StorageProviderBaidu   StorageProvider = StorageProvider(domain.StorageProviderBaidu)
	StorageProviderQiniu   StorageProvider = StorageProvider(domain.StorageProviderQiniu)
	StorageProviderHuawei  StorageProvider = StorageProvider(domain.StorageProviderHuawei)
	StorageProviderUCloud  StorageProvider = StorageProvider(domain.StorageProviderUCloud)
	StorageProviderMinIO   StorageProvider = StorageProvider(domain.StorageProviderMinIO)
	StorageProviderRustFS  StorageProvider = StorageProvider(domain.StorageProviderRustFS)
)

// RequestMeta 携带一次应用服务调用的认证和本地化信息。
type RequestMeta struct {
	Token  string `json:"token"`
	Locale Locale `json:"locale"`
}

// InstallationStatus 定义企业初始化状态和公开企业名称。
type InstallationStatus struct {
	Installed        bool   `json:"installed"`
	OrganizationName string `json:"organizationName"`
}

// InstallWorkspaceInput 定义企业初始化输入。
type InstallWorkspaceInput struct {
	OrganizationName string `json:"organizationName"`
	DisplayName      string `json:"displayName"`
	Email            string `json:"email"`
	Password         string `json:"password"`
	Locale           Locale `json:"locale"`
	TimeZone         string `json:"timeZone"`
}

// LoginInput 定义登录输入。
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Auth 包含登录身份和访问令牌。
type Auth struct {
	Identity  Identity  `json:"identity"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Organization 定义当前企业信息。
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OrganizationInput 定义企业名称修改输入。
type OrganizationInput struct {
	Name string `json:"name"`
}

// User 定义当前企业成员信息。
type User struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Email          string     `json:"email"`
	DisplayName    string     `json:"displayName"`
	Role           UserRole   `json:"role"`
	Status         UserStatus `json:"status"`
	Locale         Locale     `json:"locale"`
	TimeZone       string     `json:"timeZone"`
	WorkStatus     WorkStatus `json:"workStatus"`
	AvatarURL      string     `json:"avatarUrl"`
}

// Identity 定义当前用户及其所属企业。
type Identity struct {
	Organization Organization `json:"organization"`
	User         User         `json:"user"`
}

// ProfileInput 定义当前用户可编辑的个人资料字段。
type ProfileInput struct {
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	AvatarFileID string `json:"avatarFileId"`
}

// FilePurpose 表示文件上传用途。
type FilePurpose string

const (
	FilePurposeUserAvatar FilePurpose = FilePurpose(domain.FilePurposeUserAvatar)
)

// FileUploadInput 定义创建上传所需的文件元数据。
type FileUploadInput struct {
	Purpose     FilePurpose `json:"purpose"`
	FileName    string      `json:"fileName"`
	ContentType string      `json:"contentType"`
	ByteSize    int64       `json:"byteSize"`
}

// File 定义前端可使用的文件元数据。
type File struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	ByteSize    int64  `json:"byteSize"`
	ContentURL  string `json:"contentUrl"`
}

// FileUploadRequest 定义客户端上传文件内容所需的 HTTP 请求。
type FileUploadRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// FileUpload 包含待上传文件和内容上传请求。
type FileUpload struct {
	File    File              `json:"file"`
	Request FileUploadRequest `json:"request"`
}

// ProfileImageFile 定义原生端选择的用户头像文件。
type ProfileImageFile struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	DataBase64  string `json:"dataBase64"`
}

// ChangePasswordInput 定义当前用户修改密码所需字段。
type ChangePasswordInput struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// UserPreferencesInput 定义当前用户的语言和时区设置。
type UserPreferencesInput struct {
	Locale   Locale `json:"locale"`
	TimeZone string `json:"timeZone"`
}

// UserWorkStatusInput 定义当前用户主动设置的工作状态。
type UserWorkStatusInput struct {
	WorkStatus WorkStatus `json:"workStatus"`
}

// Conversation 定义收件箱中的会话。
type Conversation struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Initials string    `json:"initials"`
	Channel  string    `json:"channel"`
	Preview  string    `json:"preview"`
	Time     string    `json:"time"`
	Status   string    `json:"status"`
	Unread   int       `json:"unread,omitempty"`
	Online   bool      `json:"online,omitempty"`
	Messages []Message `json:"messages"`
}

// Message 定义收件箱会话中的消息。
type Message struct {
	ID     string        `json:"id"`
	Author MessageAuthor `json:"author"`
	Text   string        `json:"text"`
	Time   string        `json:"time"`
}

// Inbox 定义统一收件箱结果。
type Inbox struct {
	Organization  Organization   `json:"organization"`
	User          User           `json:"user"`
	Conversations []Conversation `json:"conversations"`
}

// WebsiteChannelSummary 定义网站渠道列表项。
type WebsiteChannelSummary struct {
	ID              string      `json:"id"`
	OrganizationID  string      `json:"organizationId"`
	CreatedByUserID string      `json:"createdByUserId"`
	Type            ChannelType `json:"type"`
	Name            string      `json:"name"`
	Description     *string     `json:"description"`
	DefaultLocale   Locale      `json:"defaultLocale"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
	DeletedAt       *time.Time  `json:"deletedAt"`
}

// WebsiteChannel 定义网站渠道详情。
type WebsiteChannel struct {
	WebsiteChannelSummary
	ChatInterface WebsiteChannelChatInterface `json:"chatInterface"`
}

// WebsiteChannelInput 定义网站渠道可编辑字段。
type WebsiteChannelInput struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultLocale Locale `json:"defaultLocale"`
}

// WebsiteChannelChatInterface 定义网站渠道访客界面设置。
type WebsiteChannelChatInterface struct {
	Title           string  `json:"title"`
	Subtitle        *string `json:"subtitle"`
	GreetingMessage *string `json:"greetingMessage"`
	ThemeColor      string  `json:"themeColor"`
}

// WebsiteChannelChatInterfaceInput 定义网站渠道访客界面输入。
type WebsiteChannelChatInterfaceInput struct {
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	GreetingMessage string `json:"greetingMessage"`
	ThemeColor      string `json:"themeColor"`
}

// ChannelSummary 定义渠道选择项。
type ChannelSummary struct {
	ID   string      `json:"id"`
	Type ChannelType `json:"type"`
	Name string      `json:"name"`
}

// ChannelList 定义渠道选择项列表。
type ChannelList struct {
	Channels []ChannelSummary `json:"channels"`
}

// WebsiteChannelList 定义网站渠道列表。
type WebsiteChannelList struct {
	Channels []WebsiteChannelSummary `json:"channels"`
}

// PageInfo 定义分页信息。
type PageInfo struct {
	Number int `json:"number"`
	Size   int `json:"size"`
	Total  int `json:"total"`
}

// UserListInput 定义企业成员列表查询条件。
type UserListInput struct {
	Query    string      `json:"query"`
	Status   *UserStatus `json:"status,omitempty"`
	Role     *UserRole   `json:"role,omitempty"`
	TeamID   string      `json:"teamId"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

// CreateUserInput 定义新增企业成员字段。
type CreateUserInput struct {
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	Role        UserRole `json:"role"`
	TeamIDs     []string `json:"teamIds"`
}

// UpdateDirectoryUserInput 定义企业成员可编辑字段。
type UpdateDirectoryUserInput struct {
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Role        UserRole `json:"role"`
	TeamIDs     []string `json:"teamIds"`
}

// TeamSummary 定义团队选择项和成员所属团队字段。
type TeamSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DirectoryUser 定义企业成员目录字段。
type DirectoryUser struct {
	ID          string        `json:"id"`
	Email       string        `json:"email"`
	DisplayName string        `json:"displayName"`
	Role        UserRole      `json:"role"`
	Status      UserStatus    `json:"status"`
	WorkStatus  WorkStatus    `json:"workStatus"`
	Teams       []TeamSummary `json:"teams"`
	CreatedAt   time.Time     `json:"createdAt"`
}

// UserList 定义企业成员分页结果。
type UserList struct {
	Users []DirectoryUser `json:"users"`
	Page  PageInfo        `json:"page"`
}

// TeamInput 定义团队可编辑字段。
type TeamInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Team 定义团队详情。
type Team struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MemberCount int       `json:"memberCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TeamListInput 定义团队列表查询条件。
type TeamListInput struct {
	Query    string `json:"query"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// TeamList 定义团队分页结果。
type TeamList struct {
	Teams []Team   `json:"teams"`
	Page  PageInfo `json:"page"`
}

// ContactMethodInput 定义联系人联系方式输入。
type ContactMethodInput struct {
	Type      ContactMethodType `json:"type"`
	Value     string            `json:"value"`
	Label     string            `json:"label"`
	IsPrimary bool              `json:"isPrimary"`
}

// ContactInput 定义联系人可编辑字段。
type ContactInput struct {
	DisplayName string               `json:"displayName"`
	ChannelID   string               `json:"channelId"`
	Stage       ContactStage         `json:"stage"`
	Notes       string               `json:"notes"`
	Methods     []ContactMethodInput `json:"methods"`
}

// ContactListInput 定义联系人列表查询条件。
type ContactListInput struct {
	Query      string             `json:"query"`
	Stage      *ContactStage      `json:"stage,omitempty"`
	ChannelID  string             `json:"channelId"`
	MethodType *ContactMethodType `json:"methodType,omitempty"`
	Sort       ContactSort        `json:"sort"`
	Page       int                `json:"page"`
	PageSize   int                `json:"pageSize"`
	Deleted    bool               `json:"deleted"`
}

// ContactSummary 定义联系人列表项。
type ContactSummary struct {
	ID                string       `json:"id"`
	DisplayName       *string      `json:"displayName"`
	Stage             ContactStage `json:"stage"`
	PrimaryEmail      *string      `json:"primaryEmail"`
	PrimaryPhone      *string      `json:"primaryPhone"`
	SourceChannelName string       `json:"sourceChannelName"`
	CreatedAt         time.Time    `json:"createdAt"`
	DeletedAt         *time.Time   `json:"deletedAt"`
}

// ContactRecord 定义联系人详情字段。
type ContactRecord struct {
	ID              string       `json:"id"`
	SourceChannelID string       `json:"sourceChannelId"`
	DisplayName     *string      `json:"displayName"`
	Stage           ContactStage `json:"stage"`
	Notes           *string      `json:"notes"`
	CreatedAt       time.Time    `json:"createdAt"`
}

// ContactMethod 定义联系人联系方式。
type ContactMethod struct {
	Type      ContactMethodType `json:"type"`
	Value     string            `json:"value"`
	Label     *string           `json:"label"`
	IsPrimary bool              `json:"isPrimary"`
}

// ContactChannelIdentity 定义联系人渠道身份。
type ContactChannelIdentity struct {
	ChannelID   string  `json:"channelId"`
	ChannelName string  `json:"channelName"`
	ExternalID  string  `json:"externalId"`
	DisplayName *string `json:"displayName"`
}

// ContactSourceChannel 定义联系人来源渠道。
type ContactSourceChannel struct {
	ID   string      `json:"id"`
	Type ChannelType `json:"type"`
	Name string      `json:"name"`
}

// Contact 定义联系人完整详情。
type Contact struct {
	Contact           ContactRecord            `json:"contact"`
	SourceChannel     ContactSourceChannel     `json:"sourceChannel"`
	Methods           []ContactMethod          `json:"methods"`
	ChannelIdentities []ContactChannelIdentity `json:"channelIdentities"`
}

// ContactList 定义联系人分页结果。
type ContactList struct {
	Contacts []ContactSummary `json:"contacts"`
	Page     PageInfo         `json:"page"`
}

// S3Setting 定义 S3 兼容对象存储配置。
type S3Setting struct {
	Enabled         bool            `json:"enabled"`
	Provider        StorageProvider `json:"provider"`
	Endpoint        string          `json:"endpoint"`
	Region          string          `json:"region"`
	Bucket          string          `json:"bucket"`
	AccessKeyID     string          `json:"accessKeyId"`
	SecretAccessKey string          `json:"secretAccessKey"`
	ForcePathStyle  bool            `json:"forcePathStyle"`
}
