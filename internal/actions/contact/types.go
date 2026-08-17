//go:build server

package contact

import (
	"time"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

const (
	// StageVisitor 表示尚未形成明确意向的外部联系人。
	StageVisitor = "visitor"
	// StageLead 表示具有潜在线索价值的外部联系人。
	StageLead = "lead"
	// StageCustomer 表示已经成为客户的外部联系人。
	StageCustomer = "customer"
	// MethodEmail 表示邮箱联系方式。
	MethodEmail = "email"
	// MethodPhone 表示电话联系方式。
	MethodPhone = "phone"
)

// MethodInput 定义联系人联系方式输入。
type MethodInput struct {
	Type      string
	Value     string
	Label     string
	IsPrimary bool
}

// ContactInput 定义外部联系人可编辑字段。
type ContactInput struct {
	DisplayName string
	ChannelID   string
	Stage       string
	Notes       string
	Methods     []MethodInput
}

// ListInput 定义外部联系人列表查询条件。
type ListInput struct {
	Query      string
	Stage      string
	ChannelID  string
	MethodType string
	Sort       string
	Page       int
	PageSize   int
	Deleted    bool
}

// PageInfo 定义服务端分页信息。
type PageInfo struct {
	Number int `json:"number"`
	Size   int `json:"size"`
	Total  int `json:"total"`
}

// ContactSummary 定义外部联系人列表项。
type ContactSummary struct {
	ID                string     `bun:"id" json:"id"`
	DisplayName       *string    `bun:"display_name" json:"displayName"`
	Stage             string     `bun:"stage" json:"stage"`
	Notes             *string    `bun:"notes" json:"notes"`
	PrimaryEmail      *string    `bun:"primary_email" json:"primaryEmail"`
	PrimaryPhone      *string    `bun:"primary_phone" json:"primaryPhone"`
	SourceChannelName *string    `bun:"source_channel_name" json:"sourceChannelName"`
	ChannelCount      int        `bun:"channel_count" json:"channelCount"`
	CreatedAt         time.Time  `bun:"created_at" json:"createdAt"`
	UpdatedAt         time.Time  `bun:"updated_at" json:"updatedAt"`
	DeletedAt         *time.Time `bun:"deleted_at" json:"deletedAt"`
}

// ChannelIdentity 定义联系人渠道身份及渠道摘要。
type ChannelIdentity struct {
	ID          string     `bun:"id" json:"id"`
	ChannelID   string     `bun:"channel_id" json:"channelId"`
	ChannelType string     `bun:"channel_type" json:"channelType"`
	ChannelName string     `bun:"channel_name" json:"channelName"`
	ExternalID  string     `bun:"external_id" json:"externalId"`
	DisplayName *string    `bun:"display_name" json:"displayName"`
	LastSeenAt  *time.Time `bun:"last_seen_at" json:"lastSeenAt"`
}

// SourceChannel 定义联系人首次添加或接入时的来源渠道。
type SourceChannel struct {
	ID   string `bun:"id" json:"id"`
	Type string `bun:"type" json:"type"`
	Name string `bun:"name" json:"name"`
}

// ContactDetail 定义外部联系人完整详情。
type ContactDetail struct {
	Contact           servermodels.Contact         `json:"contact"`
	SourceChannel     *SourceChannel               `json:"sourceChannel"`
	Methods           []servermodels.ContactMethod `json:"methods"`
	ChannelIdentities []ChannelIdentity            `json:"channelIdentities"`
}

// ListOutput 定义外部联系人分页结果。
type ListOutput struct {
	Contacts []ContactSummary `json:"contacts"`
	Page     PageInfo         `json:"page"`
}
