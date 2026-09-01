//go:build server

package integrationconnection

import (
	"context"
	"errors"
	"fmt"
	"time"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
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
	if !common.ValidUUID(connectionID) {
		return TestResult{}, ErrNotFound
	}
	testErr := a.Execute(ctx, input)
	if ctx.Err() != nil {
		return TestResult{}, testErr
	}
	// 配置本身不合法说明没有发生真实探测，直接返回校验错误，不刷新连接状态。
	if _, ok := errors.AsType[*ValidationError](testErr); ok {
		return TestResult{}, testErr
	}
	status := domain.IntegrationConnectionStatusAvailable
	if testErr != nil {
		status = domain.IntegrationConnectionStatusUnavailable
	}
	testedAt := time.Now().UTC()
	updateErr := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		result, err := tx.NewUpdate().
			Model((*servermodels.IntegrationConnection)(nil)).
			Set("status = ?", status).
			Set("last_tested_at = ?", testedAt).
			Set("updated_at = now()").
			Where("id = ?", connectionID).
			Where("organization_id = ?", identity.Organization.ID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("record integration connection test: %w", err)
		}
		if affected, err := result.RowsAffected(); err == nil && affected == 0 {
			return ErrNotFound
		}
		return nil
	})
	if updateErr != nil {
		return TestResult{}, updateErr
	}
	return TestResult{Status: status, TestedAt: testedAt}, testErr
}
