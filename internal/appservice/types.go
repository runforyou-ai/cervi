// Package appservice 定义跨平台应用服务及其传输契约。
//
// 各业务领域的传输类型按领域拆分在 types_*.go 中，本文件只保留
// 会话、安装和跨领域共用的通用契约。
package appservice

import (
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// Locale 表示应用支持的本地化语言。
type Locale string

const (
	LocaleChineseSimplified   Locale = Locale(domain.LocaleChineseSimplified)
	LocaleEnglishUnitedStates Locale = Locale(domain.LocaleEnglishUnitedStates)
)

// CustomerLocale 表示面向客户的界面、系统话术与通知邮件支持的语言。
type CustomerLocale string

const (
	CustomerLocaleChineseSimplified   CustomerLocale = CustomerLocale(domain.CustomerLocaleChineseSimplified)
	CustomerLocaleEnglishUnitedStates CustomerLocale = CustomerLocale(domain.CustomerLocaleEnglishUnitedStates)
	CustomerLocaleHindiIndia          CustomerLocale = CustomerLocale(domain.CustomerLocaleHindiIndia)
)

// SessionState 表示会话入口。
type SessionState string

const (
	SessionStateReady     SessionState = "ready"
	SessionStateLogin     SessionState = "login"
	SessionStateSetup     SessionState = "setup"
	SessionStateConnect   SessionState = "connect"
	SessionStateWorkspace SessionState = "workspace"
)

// DeploymentMode 表示服务端部署形态。
type DeploymentMode string

const (
	DeploymentModeSelfHosted DeploymentMode = DeploymentMode(domain.DeploymentModeSelfHosted)
	DeploymentModeManaged    DeploymentMode = DeploymentMode(domain.DeploymentModeManaged)
)

// ErrorKind 表示业务失败种类。
type ErrorKind string

const (
	ErrorKindInvalid     ErrorKind = "invalid"
	ErrorKindNotFound    ErrorKind = "not_found"
	ErrorKindConflict    ErrorKind = "conflict"
	ErrorKindUnavailable ErrorKind = "unavailable"
	ErrorKindFailed      ErrorKind = "failed"
)

// NotificationPermissionStatus 表示当前设备的系统通知授权状态。
type NotificationPermissionStatus string

const (
	NotificationPermissionStatusPrompt      NotificationPermissionStatus = "prompt"
	NotificationPermissionStatusGranted     NotificationPermissionStatus = "granted"
	NotificationPermissionStatusDenied      NotificationPermissionStatus = "denied"
	NotificationPermissionStatusUnsupported NotificationPermissionStatus = "unsupported"
)

// Startup 表示应用启动入口和服务端部署形态，登录页按部署形态选择登录方式。
type Startup struct {
	State          SessionState   `json:"state"`
	DeploymentMode DeploymentMode `json:"deploymentMode,omitempty"`
}

// DeviceHeader 是设备运行期调用携带本机设备编号的请求头。
const DeviceHeader = "X-Cervi-Device"

// WorkspaceHeader 是工作区级调用携带目标工作区编号的请求头。
const WorkspaceHeader = "X-Cervi-Workspace"

// RequestMeta 携带一次应用服务调用的认证、目标工作区和本地化信息；WorkspaceID 经 WorkspaceHeader 传输，
// DeviceID 只由原生端设备进程设置，经 DeviceHeader 传输。
type RequestMeta struct {
	Token       string `json:"token"`
	WorkspaceID string `json:"workspaceId"`
	Locale      Locale `json:"locale"`
	DeviceID    string `json:"-"`
}

// InstallationStatus 定义部署是否已完成首次安装、是否开放注册和服务端部署形态。
type InstallationStatus struct {
	Installed        bool           `json:"installed"`
	RegistrationOpen bool           `json:"registrationOpen"`
	DeploymentMode   DeploymentMode `json:"deploymentMode"`
}

// InstallWorkspaceInput 定义首次安装输入：部署管理员账号和第一个工作区。
type InstallWorkspaceInput struct {
	WorkspaceName string `json:"workspaceName"`
	WorkspaceSlug string `json:"workspaceSlug"`
	DisplayName   string `json:"displayName"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	Locale        Locale `json:"locale"`
	TimeZone      string `json:"timeZone"`
}

// RegisterInput 定义注册本地账号的输入。
type RegisterInput struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Locale      Locale `json:"locale"`
	TimeZone    string `json:"timeZone"`
}

// LoginInput 定义登录输入。
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// OfficialLoginInput 定义发起官方账号登录的输入；state、nonce 与 PKCE verifier 由客户端生成并保存。
type OfficialLoginInput struct {
	State         string `json:"state"`
	Nonce         string `json:"nonce"`
	CodeChallenge string `json:"codeChallenge"`
}

// OfficialLoginStart 返回官方账号登录尝试编号和授权地址。
type OfficialLoginStart struct {
	AttemptID        string `json:"attemptId"`
	AuthorizationURL string `json:"authorizationUrl"`
}

// OfficialLoginCompletion 定义用授权码完成官方账号登录的输入。
type OfficialLoginCompletion struct {
	AttemptID    string `json:"attemptId"`
	Code         string `json:"code"`
	CodeVerifier string `json:"codeVerifier"`
}

// Auth 包含登录账号和会话令牌。
type Auth struct {
	Account   Account   `json:"account"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Account 定义当前登录账号。
type Account struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	DisplayName       string `json:"displayName"`
	Locale            Locale `json:"locale"`
	TimeZone          string `json:"timeZone"`
	IsDeploymentAdmin bool   `json:"isDeploymentAdmin"`
}

// Workspace 定义账号可进入的工作区。
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// WorkspaceList 定义账号可进入的全部工作区。
type WorkspaceList struct {
	Items []Workspace `json:"items"`
}

// WorkspaceInput 定义新建工作区的名称和标识。
type WorkspaceInput struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// DeploymentSettings 定义部署级设置。
type DeploymentSettings struct {
	RegistrationOpen bool `json:"registrationOpen"`
}

// Identity 定义当前成员及其所在工作区。
type Identity struct {
	Organization Organization `json:"organization"`
	User         CurrentUser  `json:"user"`
}

// ConversationWindowInput 定义桌面端打开会话独立窗口的输入，WorkspaceSlug 是会话所在工作区的标识。
type ConversationWindowInput struct {
	WorkspaceSlug  string `json:"workspaceSlug"`
	ConversationID string `json:"conversationId"`
	Title          string `json:"title"`
}

// MessageNotificationInput 定义当前设备的新消息通知内容。
type MessageNotificationInput struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	SoundEnabled bool   `json:"soundEnabled"`
}

// PageInfo 定义分页信息。
type PageInfo struct {
	Number int `json:"number"`
	Size   int `json:"size"`
	Total  int `json:"total"`
}

// WorkspaceURL 返回部署地址下进入工作区的 Web 地址。
func WorkspaceURL(publicURL, slug string) string {
	return strings.TrimRight(publicURL, "/") + "/#/w/" + slug
}
