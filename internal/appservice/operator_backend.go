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
// operator_direct_backend_gen.go 和 internal/api/operator_service_gen.go。
// 指令不接受 auth 和 manual 选项：运营调用一律先校验运营服务凭据，运营接口
// 也不进入 Service 委托、API Proxy 和 Wails 绑定。
//
// 本契约面向 SaaS 后端的服务间调用，Backend 面向各端客户端，新增方法按消费者
// 归入其中一个，不跨契约暴露。运营请求的目标企业只取自路径或请求体中显式给出
// 的企业编号，不按访问地址推导。
type OperatorBackend interface {
	// LoadDeployment 返回部署形态与企业域名后缀。
	//cervi:route GET /deployment
	LoadDeployment(context.Context, OperatorRequestMeta) (OperatorDeployment, error)
}
