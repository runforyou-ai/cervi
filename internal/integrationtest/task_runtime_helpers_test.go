//go:build server

package integrationtest

import (
	"context"
	"testing"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	"github.com/runforyou-ai/cervi/internal/actions/serviceassignment"
	"github.com/runforyou-ai/cervi/internal/actions/servicesummary"
	"github.com/runforyou-ai/cervi/internal/actions/servicetimeout"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// newTestTasks 创建不连接 NATS 的任务运行时，并以空处理器注册客服处理周期分配与转人工承接任务，供业务事务投递。
func newTestTasks(db *bun.DB) *servertask.Runtime {
	tasks := servertask.New(db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(serviceassignment.AssignActionName, func(context.Context, serviceassignment.AssignInput) error { return nil }); err != nil {
		panic(err)
	}
	if err := tasks.Registry().RegisterJSON(serviceassignment.BackfillActionName, func(context.Context, serviceassignment.BackfillInput) error { return nil }); err != nil {
		panic(err)
	}
	if err := tasks.Registry().RegisterJSON(agentrunaction.ReturnedHandoffActionName, func(context.Context, agentrunaction.ReturnedHandoffInput) error { return nil }); err != nil {
		panic(err)
	}
	if err := tasks.Registry().RegisterJSON(servicetimeout.ProcessActionName, func(context.Context, servicetimeout.ProcessInput) error { return nil }); err != nil {
		panic(err)
	}
	if err := tasks.Registry().RegisterJSON(servicesummary.SummarizeActionName, func(context.Context, servicesummary.SummarizeInput) error { return nil }); err != nil {
		panic(err)
	}
	if err := tasks.Registry().RegisterJSON(servicesummary.HandoffSummaryActionName, func(context.Context, servicesummary.HandoffSummaryInput) error { return nil }); err != nil {
		panic(err)
	}
	return tasks
}

// disableAutoAssignment 把企业内全部成员的最大接待量设为 0，使只验证路由去向的测试不触发自动分配。
func disableAutoAssignment(t *testing.T, db *bun.DB, organizationID string) {
	t.Helper()
	if _, err := db.NewUpdate().Table("users").Set("max_service_sessions = 0").Where("organization_id = ?", organizationID).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
}
