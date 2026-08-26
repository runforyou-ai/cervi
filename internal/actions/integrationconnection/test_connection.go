//go:build server

package integrationconnection

import (
	"context"
	"fmt"
	"time"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	"github.com/runforyou-ai/cervi/internal/integration/connector"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// TestConnectionAction 测试连接器配置并记录已保存连接的状态。
type TestConnectionAction struct {
	db       *bun.DB
	runner   *connectiontest.Runner
	registry *connector.Registry
}

// NewTestConnectionAction 创建连接器测试操作。
func NewTestConnectionAction(db *bun.DB, runner *connectiontest.Runner, registry *connector.Registry) *TestConnectionAction {
	return &TestConnectionAction{db: db, runner: runner, registry: registry}
}

// Execute 校验连接器草稿并执行只读探测。
func (a *TestConnectionAction) Execute(ctx context.Context, input ConnectionInput) error {
	input, fields := normalizeConnectionInput(input)
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	probe, err := a.registry.NewProbe(connector.Config{
		Type: input.Type, APIURL: input.Configuration.APIURL, APIKey: input.Configuration.APIKey,
	})
	if err == nil {
		err = a.runner.Run(ctx, connectiontest.Target{
			Category: connectiontest.CategoryConnector,
			Adapter:  string(input.Type),
			Location: connectiontest.LocationServer,
		}, probe)
	}
	return err
}

// ExecuteAndRecord 测试配置并记录当前企业中已保存连接器的状态。
func (a *TestConnectionAction) ExecuteAndRecord(ctx context.Context, identity *servermodels.Identity, connectionID string, input ConnectionInput) (TestResult, error) {
	if err := identityaction.Validate(ctx, a.db, identity); err != nil {
		return TestResult{}, err
	}
	testErr := a.Execute(ctx, input)
	if ctx.Err() != nil {
		return TestResult{}, testErr
	}
	status := domain.IntegrationConnectionStatusAvailable
	if testErr != nil {
		status = domain.IntegrationConnectionStatusUnavailable
	}
	testedAt := time.Now().UTC()
	if _, updateErr := a.db.NewUpdate().
		Model((*servermodels.IntegrationConnection)(nil)).
		Set("status = ?", status).
		Set("last_tested_at = ?", testedAt).
		Where("id = ?", connectionID).
		Where("organization_id = ?", identity.Organization.ID).
		Exec(ctx); updateErr != nil {
		return TestResult{}, fmt.Errorf("record integration connection test: %w", updateErr)
	}
	return TestResult{Status: status, TestedAt: testedAt}, testErr
}
