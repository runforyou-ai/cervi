package appservice

import "github.com/runforyou-ai/cervi/internal/domain"

// OrganizationIdentityType 表示企业身份类型。
type OrganizationIdentityType string

const (
	OrganizationIdentityTypeUser  OrganizationIdentityType = OrganizationIdentityType(domain.OrganizationIdentityTypeUser)
	OrganizationIdentityTypeAgent OrganizationIdentityType = OrganizationIdentityType(domain.OrganizationIdentityTypeAgent)
)

// Organization 定义当前企业及其通用设置。
type Organization struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	AllowArbitraryURL bool   `json:"allowArbitraryUrl"`
}

// OrganizationInput 定义企业通用设置修改输入。
type OrganizationInput struct {
	Name              string `json:"name"`
	AllowArbitraryURL bool   `json:"allowArbitraryUrl"`
}
