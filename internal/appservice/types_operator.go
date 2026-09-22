//go:build server

package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// OperatorRequestMeta 描述运营调用的服务凭据、请求关联标识和语言。
type OperatorRequestMeta struct {
	Credential string
	RequestID  string
	Locale     Locale
}

// OperatorIdentity 表示已通过凭据校验的运营调用方。
type OperatorIdentity struct {
	RequestID string
}

// OperatorDeployment 描述本部署的形态与企业域名后缀。
type OperatorDeployment struct {
	Mode                DeploymentMode `json:"mode"`
	ManagedDomainSuffix string         `json:"managedDomainSuffix"`
}

// OrganizationLifecycleStatus 表示企业生命周期状态。
type OrganizationLifecycleStatus string

const (
	OrganizationLifecycleActive    OrganizationLifecycleStatus = OrganizationLifecycleStatus(domain.OrganizationLifecycleActive)
	OrganizationLifecycleSuspended OrganizationLifecycleStatus = OrganizationLifecycleStatus(domain.OrganizationLifecycleSuspended)
	OrganizationLifecycleDeleting  OrganizationLifecycleStatus = OrganizationLifecycleStatus(domain.OrganizationLifecycleDeleting)
	OrganizationLifecycleDeleted   OrganizationLifecycleStatus = OrganizationLifecycleStatus(domain.OrganizationLifecycleDeleted)
)

// OperatorDomainAvailabilityInput 定义域名前缀可用性查询条件。
type OperatorDomainAvailabilityInput struct {
	Prefix string `json:"prefix" query:"prefix"`
}

// OperatorDomainAvailability 描述域名前缀在查询时刻的可用状态。
type OperatorDomainAvailability struct {
	DomainPrefix string `json:"domainPrefix"`
	AccessHost   string `json:"accessHost"`
	PublicURL    string `json:"publicUrl"`
	Available    bool   `json:"available"`
}

// OperatorInitialUserInput 定义开通企业的初始成员，Subject 是官方身份服务中的稳定账号标识。
type OperatorInitialUserInput struct {
	Subject     string `json:"subject"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Locale      Locale `json:"locale"`
	TimeZone    string `json:"timeZone"`
}

// OperatorEntitlementInput 定义服务权益快照，ServiceEndsAt 为空表示长期有效。
type OperatorEntitlementInput struct {
	Revision      int64      `json:"revision"`
	PlanCode      string     `json:"planCode"`
	ServiceEndsAt *time.Time `json:"serviceEndsAt"`
}

// OperatorProvisionInput 定义运营开通企业请求。
type OperatorProvisionInput struct {
	ProvisioningID   string                   `json:"provisioningId"`
	OrganizationName string                   `json:"organizationName"`
	DomainPrefix     string                   `json:"domainPrefix"`
	InitialUser      OperatorInitialUserInput `json:"initialUser"`
	Entitlement      OperatorEntitlementInput `json:"entitlement"`
}

// OperatorProvisioning 描述开通操作创建的企业及其当前状态。
type OperatorProvisioning struct {
	ProvisioningID      string                      `json:"provisioningId"`
	OrganizationID      string                      `json:"organizationId"`
	AccessHost          string                      `json:"accessHost"`
	PublicURL           string                      `json:"publicUrl"`
	InitialUserID       string                      `json:"initialUserId"`
	LifecycleStatus     OrganizationLifecycleStatus `json:"lifecycleStatus"`
	EntitlementRevision int64                       `json:"entitlementRevision"`
}

// OperatorOrganizationListInput 定义运营企业列表的筛选与分页条件，Query 匹配企业名称或访问地址。
type OperatorOrganizationListInput struct {
	Query           string                       `json:"query" query:"query"`
	LifecycleStatus *OrganizationLifecycleStatus `json:"lifecycleStatus,omitempty" query:"lifecycleStatus"`
	Page            int                          `json:"page" query:"page,default=1"`
	PageSize        int                          `json:"pageSize" query:"pageSize,default=50"`
}

// OperatorEntitlement 描述企业当前生效的服务权益，Valid 由服务截止时间计算。
type OperatorEntitlement struct {
	Revision      int64      `json:"revision"`
	PlanCode      string     `json:"planCode"`
	ServiceEndsAt *time.Time `json:"serviceEndsAt"`
	Valid         bool       `json:"valid"`
	AppliedAt     time.Time  `json:"appliedAt"`
}

// OperatorOrganization 描述运营侧查看的企业摘要；未经运营开通的企业没有开通标识和权益。
type OperatorOrganization struct {
	ID              string                      `json:"id"`
	Name            string                      `json:"name"`
	AccessHost      string                      `json:"accessHost"`
	PublicURL       string                      `json:"publicUrl"`
	LifecycleStatus OrganizationLifecycleStatus `json:"lifecycleStatus"`
	ProvisioningID  *string                     `json:"provisioningId"`
	Entitlement     *OperatorEntitlement        `json:"entitlement"`
	CreatedAt       time.Time                   `json:"createdAt"`
}

// OperatorOrganizationList 是一页企业摘要及符合条件的总数。
type OperatorOrganizationList struct {
	Items []OperatorOrganization `json:"items"`
	Total int                    `json:"total"`
}
