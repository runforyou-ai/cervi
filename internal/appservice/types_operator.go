//go:build server

package appservice

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
