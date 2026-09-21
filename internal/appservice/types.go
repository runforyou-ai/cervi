// Package appservice 定义跨平台应用服务及其传输契约。
//
// 各业务领域的传输类型按领域拆分在 types_*.go 中，本文件只保留
// 会话、安装和跨领域共用的通用契约。
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
	SessionStateReady          SessionState = "ready"
	SessionStateLogin          SessionState = "login"
	SessionStateSetup          SessionState = "setup"
	SessionStateConnect        SessionState = "connect"
	SessionStateInvalidAddress SessionState = "invalid_address"
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

// Startup 表示应用启动入口和企业名称。
type Startup struct {
	State            SessionState `json:"state"`
	OrganizationName string       `json:"organizationName,omitempty"`
}

// DeviceHeader 是设备运行期调用携带本机设备编号的请求头。
const DeviceHeader = "X-Cervi-Device"

// RequestMeta 携带一次应用服务调用的认证和本地化信息；DeviceID 只由原生端设备进程设置，经 DeviceHeader 传输。
type RequestMeta struct {
	Token    string `json:"token"`
	Locale   Locale `json:"locale"`
	DeviceID string `json:"-"`
}

// InstallationStatus 定义企业初始化状态、公开企业名称和服务端部署形态。
type InstallationStatus struct {
	Installed        bool           `json:"installed"`
	OrganizationName string         `json:"organizationName"`
	DeploymentMode   DeploymentMode `json:"deploymentMode"`
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

// Identity 定义当前用户及其所属企业。
type Identity struct {
	Organization Organization `json:"organization"`
	User         CurrentUser  `json:"user"`
}

// ConversationWindowInput 定义桌面端打开会话独立窗口的输入。
type ConversationWindowInput struct {
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
