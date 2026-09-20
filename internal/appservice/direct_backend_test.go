//go:build server

package appservice

import (
	"context"
	"errors"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/tenant"
)

// TestDirectBackendPreservesCancellation 验证请求取消原因的透传。
func TestDirectBackendPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (&directOperations{}).contactError(ctx, RequestMeta{}, errors.New("query failed"), cervii18n.ErrorContactReadFailed)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

// TestManagedDeploymentClosesInstallation 验证托管部署关闭公开初始化入口。
func TestManagedDeploymentClosesInstallation(t *testing.T) {
	ops := &directOperations{sessionGuard: sessionGuard{deploymentMode: domain.DeploymentModeManaged}}
	_, err := ops.InstallWorkspace(context.Background(), RequestMeta{}, InstallWorkspaceInput{})
	if SessionStateOf(err) != SessionStateInvalidAddress {
		t.Fatalf("error = %v, want invalid address session state", err)
	}
}

// TestManagedDeploymentRejectsUnknownAccessHost 验证托管部署把未登记访问地址收敛为企业地址无效。
func TestManagedDeploymentRejectsUnknownAccessHost(t *testing.T) {
	guard := sessionGuard{deploymentMode: domain.DeploymentModeManaged, resolveTenant: unknownTenantResolver{}}
	_, err := guard.requireInitialized(context.Background(), RequestMeta{})
	if SessionStateOf(err) != SessionStateInvalidAddress {
		t.Fatalf("managed error = %v, want invalid address session state", err)
	}

	guard.deploymentMode = domain.DeploymentModeSelfHosted
	_, err = guard.requireInitialized(context.Background(), RequestMeta{})
	if SessionStateOf(err) != SessionStateSetup {
		t.Fatalf("self hosted error = %v, want setup session state", err)
	}
}

// unknownTenantResolver 把任意访问地址解析为未登记企业。
type unknownTenantResolver struct{}

// Resolve 返回企业未登记。
func (unknownTenantResolver) Resolve(context.Context, string) (tenant.Scope, error) {
	return tenant.Scope{}, tenant.ErrNotFound
}
