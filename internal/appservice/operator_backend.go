//go:build server

package appservice

import "context"

// OperatorBackend 定义官方托管的运营管理调用。
//
// 每个方法必须携带一条 cervi:route 指令，格式为：
//
//	cervi:route <HTTP方法> <路径> [status=201] [query=<参数名>]
//
// appservicegen 按指令生成运营认证分发和 Gin 路由与 Handler，生成结果写入
// operator_direct_backend_gen.go 和 internal/api/operator_service_gen.go，
// 生成范围固定为这两层。指令只接受 status 和 query 选项，每个运营调用先校验
// 运营服务凭据再执行业务实现。
//
// 本契约的消费者是 SaaS 后端的服务间调用，各端客户端的业务调用属于 Backend，
// 新增方法按消费者归入其中一个。运营请求的目标企业只取自路径或请求体中显式
// 给出的企业编号。业务实现把领域错误转成带稳定错误码的 OperatorError。
type OperatorBackend interface {
	// LoadDeployment 返回部署形态与企业域名后缀。
	//cervi:route GET /deployment
	LoadDeployment(context.Context, OperatorRequestMeta) (OperatorDeployment, error)
	// CheckDomainAvailability 返回域名前缀在查询时刻是否可用。
	//cervi:route GET /domains/availability
	CheckDomainAvailability(context.Context, OperatorRequestMeta, OperatorDomainAvailabilityInput) (OperatorDomainAvailability, error)
	// ProvisionOrganization 按开通标识幂等地创建企业、初始成员和初始权益。
	//cervi:route POST /organizations status=201
	ProvisionOrganization(context.Context, OperatorRequestMeta, OperatorProvisionInput) (OperatorProvisioning, error)
	// GetProvisioning 返回开通标识对应企业的当前状态。
	//cervi:route GET /provisionings/:provisioningID
	GetProvisioning(context.Context, OperatorRequestMeta, string) (OperatorProvisioning, error)
	// ListOrganizations 按条件分页返回企业摘要。
	//cervi:route GET /organizations
	ListOrganizations(context.Context, OperatorRequestMeta, OperatorOrganizationListInput) (OperatorOrganizationList, error)
	// GetOrganization 返回企业摘要与状态。
	//cervi:route GET /organizations/:organizationID
	GetOrganization(context.Context, OperatorRequestMeta, string) (OperatorOrganization, error)
}
