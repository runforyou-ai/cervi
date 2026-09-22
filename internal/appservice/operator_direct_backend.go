//go:build server

package appservice

import (
	"context"
	"crypto/subtle"

	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	provisioningaction "github.com/runforyou-ai/cervi/internal/actions/provisioning"
	"github.com/uptrace/bun"
)

var _ OperatorBackend = (*OperatorDirectBackend)(nil)

// operatorGuard 校验运营服务凭据。
type operatorGuard struct {
	credential string
}

// authenticate 按常量时间比较运营服务凭据，凭据是运营接口的唯一访问控制手段。
func (g operatorGuard) authenticate(_ context.Context, meta OperatorRequestMeta) (OperatorIdentity, error) {
	if subtle.ConstantTimeCompare([]byte(meta.Credential), []byte(g.credential)) != 1 {
		return OperatorIdentity{}, invalidOperatorCredentialError(meta)
	}
	return OperatorIdentity{RequestID: meta.RequestID}, nil
}

// operatorOperations 持有已认证运营实现所需的部署配置、Action 和 Query。
type operatorOperations struct {
	operatorGuard
	deployment            OperatorDeployment
	checkDomain           *provisioningaction.CheckDomainQuery
	provisionOrganization *provisioningaction.ProvisionOrganizationAction
	getProvisioning       *provisioningaction.GetProvisioningQuery
	organizationSummaries *organizationaction.SummaryQuery
}

// OperatorConfig 定义运营后端使用的可信部署配置。
type OperatorConfig struct {
	Deployment             OperatorDeployment
	Credential             string
	OfficialIdentityIssuer string
}

// OperatorDirectBackend 校验运营凭据并把运营调用分发给已认证实现。
//
// 各 OperatorBackend 方法的认证分发由 appservicegen 生成到 operator_direct_backend_gen.go；
// 每个方法在调用实现前先取得运营身份，业务实现不重复处理认证。
type OperatorDirectBackend struct {
	ops *operatorOperations
}

// NewOperatorDirectBackend 创建直接访问服务端存储的运营后端。
func NewOperatorDirectBackend(db *bun.DB, config OperatorConfig) *OperatorDirectBackend {
	suffix := config.Deployment.ManagedDomainSuffix
	return &OperatorDirectBackend{ops: &operatorOperations{
		operatorGuard:         operatorGuard{credential: config.Credential},
		deployment:            config.Deployment,
		checkDomain:           provisioningaction.NewCheckDomainQuery(db, suffix),
		provisionOrganization: provisioningaction.NewProvisionOrganizationAction(db, config.OfficialIdentityIssuer, suffix),
		getProvisioning:       provisioningaction.NewGetProvisioningQuery(db),
		organizationSummaries: organizationaction.NewSummaryQuery(db),
	}}
}

// LoadDeployment 返回部署形态与企业域名后缀。
func (o *operatorOperations) LoadDeployment(_ context.Context, _ OperatorRequestMeta, _ OperatorIdentity) (OperatorDeployment, error) {
	return o.deployment, nil
}
