// Package domain 定义跨应用层共享且不依赖传输或存储的领域值。
package domain

// Locale 定义应用支持的本地化语言。
type Locale string

const (
	LocaleChineseSimplified   Locale = "zh-CN"
	LocaleEnglishUnitedStates Locale = "en-US"
)

// UserRole 定义企业成员角色。
type UserRole string

const (
	UserRoleOwner  UserRole = "owner"
	UserRoleMember UserRole = "member"
)

// UserStatus 定义企业成员状态。
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
)

// ChannelType 定义渠道类型。
type ChannelType string

const (
	ChannelTypeWebsite ChannelType = "website"
)

// MessageAuthor 定义消息发送方。
type MessageAuthor string

const (
	MessageAuthorVisitor MessageAuthor = "visitor"
	MessageAuthorAgent   MessageAuthor = "agent"
)

// ContactStage 定义联系人阶段。
type ContactStage string

const (
	ContactStageVisitor  ContactStage = "visitor"
	ContactStageLead     ContactStage = "lead"
	ContactStageCustomer ContactStage = "customer"
)

// ContactMethodType 定义联系人联系方式类型。
type ContactMethodType string

const (
	ContactMethodTypeEmail ContactMethodType = "email"
	ContactMethodTypePhone ContactMethodType = "phone"
)

// ContactSort 定义联系人列表排序方式。
type ContactSort string

const (
	ContactSortUpdatedAtDescending  ContactSort = "updatedAt.desc"
	ContactSortCreatedAtDescending  ContactSort = "createdAt.desc"
	ContactSortDisplayNameAscending ContactSort = "displayName.asc"
)

// StorageProvider 定义 S3 兼容对象存储提供商。
type StorageProvider string

const (
	StorageProviderGeneric StorageProvider = "generic"
	StorageProviderAWS     StorageProvider = "aws"
	StorageProviderR2      StorageProvider = "r2"
	StorageProviderAliyun  StorageProvider = "aliyun"
	StorageProviderTencent StorageProvider = "tencent"
	StorageProviderBaidu   StorageProvider = "baidu"
	StorageProviderQiniu   StorageProvider = "qiniu"
	StorageProviderHuawei  StorageProvider = "huawei"
	StorageProviderUCloud  StorageProvider = "ucloud"
	StorageProviderMinIO   StorageProvider = "minio"
	StorageProviderRustFS  StorageProvider = "rustfs"
)
