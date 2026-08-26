package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
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
//
// Deleted 由回收站路径设置，不经查询参数传输。
type ContactListInput struct {
	Query      string             `json:"query" query:"query"`
	Stage      *ContactStage      `json:"stage,omitempty" query:"stage"`
	ChannelID  string             `json:"channelId" query:"channelId"`
	MethodType *ContactMethodType `json:"methodType,omitempty" query:"methodType"`
	Sort       ContactSort        `json:"sort" query:"sort"`
	Page       int                `json:"page" query:"page,default=1"`
	PageSize   int                `json:"pageSize" query:"pageSize,default=50"`
	Deleted    bool               `json:"deleted" query:"-"`
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
