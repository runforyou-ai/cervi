//go:build server

package contact

import "time"

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
	PrimaryEmail      *string    `bun:"primary_email" json:"primaryEmail"`
	PrimaryPhone      *string    `bun:"primary_phone" json:"primaryPhone"`
	SourceChannelName string     `bun:"source_channel_name" json:"sourceChannelName"`
	CreatedAt         time.Time  `bun:"created_at" json:"createdAt"`
	DeletedAt         *time.Time `bun:"deleted_at" json:"deletedAt"`
}

// ContactRecord 定义联系人详情字段。
type ContactRecord struct {
	ID              string    `bun:"id" json:"id"`
	SourceChannelID string    `bun:"source_channel_id" json:"sourceChannelId"`
	DisplayName     *string   `bun:"display_name" json:"displayName"`
	Stage           string    `bun:"stage" json:"stage"`
	Notes           *string   `bun:"notes" json:"notes"`
	CreatedAt       time.Time `bun:"created_at" json:"createdAt"`
}

// ContactMethod 定义联系人详情中的联系方式。
type ContactMethod struct {
	Type      string  `bun:"type" json:"type"`
	Value     string  `bun:"value" json:"value"`
	Label     *string `bun:"label" json:"label"`
	IsPrimary bool    `bun:"is_primary" json:"isPrimary"`
}

// ChannelIdentity 定义联系人渠道身份及渠道摘要。
type ChannelIdentity struct {
	ChannelID   string  `bun:"channel_id" json:"channelId"`
	ChannelName string  `bun:"channel_name" json:"channelName"`
	ExternalID  string  `bun:"external_id" json:"externalId"`
	DisplayName *string `bun:"display_name" json:"displayName"`
}

// SourceChannel 定义联系人来源渠道。
type SourceChannel struct {
	ID   string `bun:"id" json:"id"`
	Type string `bun:"type" json:"type"`
	Name string `bun:"name" json:"name"`
}

// ContactDetail 定义外部联系人完整详情。
type ContactDetail struct {
	Contact           ContactRecord     `json:"contact"`
	SourceChannel     SourceChannel     `json:"sourceChannel"`
	Methods           []ContactMethod   `json:"methods"`
	ChannelIdentities []ChannelIdentity `json:"channelIdentities"`
}

// ListOutput 定义外部联系人分页结果。
type ListOutput struct {
	Contacts []ContactSummary `json:"contacts"`
	Page     PageInfo         `json:"page"`
}
