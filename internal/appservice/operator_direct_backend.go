//go:build server

package appservice

import (
	"context"
	"crypto/subtle"
	"log/slog"
)

var _ OperatorBackend = (*OperatorDirectBackend)(nil)

// operatorGuard 校验运营服务凭据。
type operatorGuard struct {
	credential string
}

// authenticate 按常量时间比较运营服务凭据，凭据是运营接口的唯一访问控制手段。
func (g operatorGuard) authenticate(_ context.Context, meta OperatorRequestMeta) (OperatorIdentity, error) {
	if subtle.ConstantTimeCompare([]byte(meta.Credential), []byte(g.credential)) != 1 {
		slog.Warn("运营凭据无效", "request_id", meta.RequestID)
		return OperatorIdentity{}, invalidOperatorCredentialError(meta)
	}
	return OperatorIdentity{RequestID: meta.RequestID}, nil
}

// operatorOperations 持有已认证运营实现所需的部署配置、Action 和 Query。
type operatorOperations struct {
	operatorGuard
	deployment OperatorDeployment
}

// OperatorDirectBackend 校验运营凭据并把运营调用分发给已认证实现。
//
// 各 OperatorBackend 方法的认证分发由 appservicegen 生成到 operator_direct_backend_gen.go；
// 每个方法在调用实现前先取得运营身份，业务实现不重复处理认证。
type OperatorDirectBackend struct {
	ops *operatorOperations
}

// NewOperatorDirectBackend 创建直接访问服务端存储的运营后端。
func NewOperatorDirectBackend(deployment OperatorDeployment, credential string) *OperatorDirectBackend {
	return &OperatorDirectBackend{ops: &operatorOperations{
		operatorGuard: operatorGuard{credential: credential},
		deployment:    deployment,
	}}
}

// LoadDeployment 返回部署形态与企业域名后缀。
func (o *operatorOperations) LoadDeployment(_ context.Context, _ OperatorRequestMeta, _ OperatorIdentity) (OperatorDeployment, error) {
	return o.deployment, nil
}
