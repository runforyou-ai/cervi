//go:build server

package integrationtest

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"uuid"

	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	mcpserveraction "github.com/runforyou-ai/cervi/internal/actions/mcpserver"
	"github.com/runforyou-ai/cervi/internal/common"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	mcpintegration "github.com/runforyou-ai/cervi/internal/integration/mcp"
	servertest "github.com/runforyou-ai/cervi/internal/servertest"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// TestMCPServerLifecycle 验证 MCP 配置的增删改查、名称唯一性和企业隔离。
func TestMCPServerLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := serverstorage.Open(ctx, servertest.DatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	identities := make([]*servermodels.Identity, 0, 2)
	for range 2 {
		installed, err := installationaction.NewInstallWorkspaceAction(db).Execute(ctx, installationaction.InstallWorkspaceInput{
			AccessHost: uuid.NewV7().String() + ".mcp.test", OrganizationName: "MCP 测试", DisplayName: "维护人员", Email: "owner@mcp.test", Password: "password123", Locale: domain.LocaleChineseSimplified, TimeZone: "UTC",
		})
		if err != nil {
			t.Fatal(err)
		}
		identities = append(identities, installed.Identity)
	}
	owner, other := identities[0], identities[1]
	client := mcpDiscoverFunc(func(context.Context, mcpintegration.Config) ([]domain.MCPTool, error) {
		return []domain.MCPTool{{Name: "search", Description: "检索文档"}}, nil
	})
	tasks := servertask.New(db, serverconfig.NATSConfig{})
	worker := mcpserveraction.NewUpdateToolsAction(db, client)
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(mcpserveraction.RefreshToolsActionName, worker.Execute, worker.FinalizeFailure); err != nil {
		t.Fatal(err)
	}
	scheduler := mcpserveraction.NewToolsScheduler(tasks)
	test := mcpserveraction.NewTestConnectionAction(client)
	create := mcpserveraction.NewCreateMCPServerAction(db, test, scheduler)
	get := mcpserveraction.NewGetMCPServerQuery(db)
	list := mcpserveraction.NewListMCPServersQuery(db)
	update := mcpserveraction.NewUpdateMCPServerAction(db, test, scheduler)
	remove := mcpserveraction.NewDeleteMCPServerAction(db)
	input := mcpserveraction.Input{Name: " Docs ", URL: " https://example.com/mcp ", ServerType: domain.MCPServerTypeStreamableHTTP, AuthorizationToken: "test-token"}
	created, err := create.Execute(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "Docs" || created.URL != "https://example.com/mcp" {
		t.Fatalf("unexpected normalization: %+v", created)
	}
	read, err := get.Execute(ctx, owner, created.ID)
	if err != nil || read.AuthorizationToken != input.AuthorizationToken || read.ServerType != input.ServerType {
		t.Fatalf("read = %+v, error = %v", read, err)
	}
	input.Name = "docs"
	_, err = create.Execute(ctx, owner, input)
	var fields *common.FieldError
	if !errors.As(err, &fields) || fields.Fields["name"] != mcpserveraction.ValidationNameDuplicate {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := create.Execute(ctx, other, input); err != nil {
		t.Fatalf("same name in other organization: %v", err)
	}
	if _, err := get.Execute(ctx, other, created.ID); !errors.Is(err, mcpserveraction.ErrNotFound) {
		t.Fatalf("cross-organization read: %v", err)
	}
	if _, err := update.Execute(ctx, other, created.ID, input); !errors.Is(err, mcpserveraction.ErrNotFound) {
		t.Fatalf("cross-organization update: %v", err)
	}
	if err := remove.Execute(ctx, other, created.ID); !errors.Is(err, mcpserveraction.ErrNotFound) {
		t.Fatalf("cross-organization delete: %v", err)
	}
	records, err := list.Execute(ctx, owner)
	if err != nil || len(records) != 1 || records[0].ID != created.ID {
		t.Fatalf("list = %+v, error = %v", records, err)
	}
	input.Name, input.URL, input.ServerType, input.AuthorizationToken = "Updated", "http://localhost:8080/sse", domain.MCPServerTypeSSE, ""
	saved, err := update.Execute(ctx, owner, created.ID, input)
	if err != nil || saved.Name != input.Name || saved.URL != input.URL || saved.ServerType != input.ServerType || saved.AuthorizationToken != "" {
		t.Fatalf("update = %+v, error = %v", saved, err)
	}
	input.Name = "Another"
	second, err := create.Execute(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	input.Name = "UPDATED"
	if _, err = update.Execute(ctx, owner, second.ID, input); !errors.As(err, &fields) || fields.Fields["name"] != mcpserveraction.ValidationNameDuplicate {
		t.Fatalf("duplicate update: %v", err)
	}
	if err := remove.Execute(ctx, owner, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := get.Execute(ctx, owner, created.ID); !errors.Is(err, mcpserveraction.ErrNotFound) {
		t.Fatalf("read deleted: %v", err)
	}
	if err := remove.Execute(ctx, owner, second.ID); err != nil {
		t.Fatal(err)
	}
	records, err = list.Execute(ctx, owner)
	if err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty list = %+v, error = %v", records, err)
	}
}

// mcpDiscoverFunc 为工具更新测试提供可控的远端响应。
type mcpDiscoverFunc func(context.Context, mcpintegration.Config) ([]domain.MCPTool, error)

// Discover 返回测试指定的工具目录或错误。
func (f mcpDiscoverFunc) Discover(ctx context.Context, config mcpintegration.Config) ([]domain.MCPTool, error) {
	return f(ctx, config)
}

// TestMCPToolsUpdates 验证保存自动投递、去重、整批替换、失败保留及旧批次拒绝。
func TestMCPToolsUpdates(t *testing.T) {
	ctx := context.Background()
	store, err := serverstorage.Open(ctx, servertest.DatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	installed, err := installationaction.NewInstallWorkspaceAction(db).Execute(ctx, installationaction.InstallWorkspaceInput{
		AccessHost: uuid.NewV7().String() + ".mcp-tools.test", OrganizationName: "工具测试", DisplayName: "维护人员", Email: "owner@mcp.test", Password: "password123", Locale: domain.LocaleChineseSimplified, TimeZone: "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	identity := installed.Identity
	tools := []domain.MCPTool{{Name: "search", Description: "查找文档"}, {Name: "read", Description: "读取文档"}}
	var discoverError error
	client := mcpDiscoverFunc(func(context.Context, mcpintegration.Config) ([]domain.MCPTool, error) { return tools, discoverError })
	tasks := servertask.New(db, serverconfig.NATSConfig{})
	worker := mcpserveraction.NewUpdateToolsAction(db, client)
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(mcpserveraction.RefreshToolsActionName, worker.Execute, worker.FinalizeFailure); err != nil {
		t.Fatal(err)
	}
	scheduler := mcpserveraction.NewToolsScheduler(tasks)
	probe := mcpserveraction.NewTestConnectionAction(client)
	create := mcpserveraction.NewCreateMCPServerAction(db, probe, scheduler)
	update := mcpserveraction.NewUpdateMCPServerAction(db, probe, scheduler)
	refresh := mcpserveraction.NewRefreshToolsAction(db, scheduler)
	get := mcpserveraction.NewGetMCPServerQuery(db)
	input := mcpserveraction.Input{Name: "Docs", URL: "https://example.com/mcp", ServerType: domain.MCPServerTypeStreamableHTTP}
	record, err := create.Execute(ctx, identity, input)
	if err != nil {
		t.Fatal(err)
	}
	if !record.ToolsUpdating || record.ToolsUpdatedAt != nil {
		t.Fatalf("create must enqueue without writing test tools: %+v", record)
	}
	first := mcpRefreshInput(t, ctx, db, record.ID)
	if err := refresh.Execute(ctx, identity); err != nil {
		t.Fatal(err)
	}
	if current := mcpRefreshInput(t, ctx, db, record.ID); current != first {
		t.Fatal("refresh duplicated pending task")
	}
	if err := worker.Execute(ctx, first); err != nil {
		t.Fatal(err)
	}
	record, err = get.Execute(ctx, identity, record.ID)
	if err != nil || record.ToolsUpdating || len(record.Tools) != 2 || record.ToolsUpdatedAt == nil {
		t.Fatalf("first refresh: %+v, %v", record, err)
	}
	// 核验每次保存的新任务标识及旧任务回调的批次校验。
	input.Name = "Renamed"
	record, err = update.Execute(ctx, identity, record.ID, input)
	if err != nil || !record.ToolsUpdating {
		t.Fatalf("save did not enqueue: %+v, %v", record, err)
	}
	second := mcpRefreshInput(t, ctx, db, record.ID)
	if second == first {
		t.Fatal("save reused old batch")
	}
	if err := worker.FinalizeFailure(ctx, first, errors.New("old worker")); err != nil {
		t.Fatal(err)
	}
	if current := mcpRefreshInput(t, ctx, db, record.ID); current != second {
		t.Fatal("old failure changed current batch")
	}
	tools = []domain.MCPTool{}
	if err := worker.Execute(ctx, second); err != nil {
		t.Fatal(err)
	}
	record, _ = get.Execute(ctx, identity, record.ID)
	if record.ToolsUpdating || record.ToolsUpdatedAt == nil || len(record.Tools) != 0 {
		t.Fatalf("empty list must replace tools: %+v", record)
	}
	// 保存探测失败时，配置和任务均保持原样。
	discoverError = connectiontest.NewError(connectiontest.StageAuthenticate, connectiontest.FailureUnauthorized, nil)
	input.Name = "Must not save"
	if _, err := update.Execute(ctx, identity, record.ID, input); err == nil {
		t.Fatal("failed probe saved configuration")
	}
	if _, err := create.Execute(ctx, identity, input); err == nil {
		t.Fatal("failed probe created configuration")
	}
	record, _ = get.Execute(ctx, identity, record.ID)
	if record.Name != "Renamed" || record.ToolsUpdating {
		t.Fatalf("failed save changed record: %+v", record)
	}
	if err := refresh.Execute(ctx, identity); err != nil {
		t.Fatal(err)
	}
	failed := mcpRefreshInput(t, ctx, db, record.ID)
	if err := worker.Execute(ctx, failed); err == nil {
		t.Fatal("expected failed discovery")
	}
	if err := worker.FinalizeFailure(ctx, failed, discoverError); err != nil {
		t.Fatal(err)
	}
	record, _ = get.Execute(ctx, identity, record.ID)
	if record.ToolsUpdating || record.ToolsFailure != string(connectiontest.FailureUnauthorized) || record.ToolsUpdatedAt == nil {
		t.Fatalf("failure did not preserve snapshot: %+v", record)
	}
	// 入队失败必须回滚保存事务。
	discoverError = nil
	brokenScheduler := mcpserveraction.NewToolsScheduler(servertask.New(db, serverconfig.NATSConfig{}))
	if _, err := mcpserveraction.NewUpdateMCPServerAction(db, probe, brokenScheduler).Execute(ctx, identity, record.ID, input); err == nil {
		t.Fatal("expected unregistered action error")
	}
	record, _ = get.Execute(ctx, identity, record.ID)
	if record.Name != "Renamed" {
		t.Fatal("enqueue failure did not roll back configuration")
	}
	// 修改连接后清除旧目录，并拒绝删除后的任务写回。
	input.URL = "https://new.example.com/mcp"
	record, err = update.Execute(ctx, identity, record.ID, input)
	if err != nil || record.ToolsUpdatedAt != nil {
		t.Fatalf("new connection retained old snapshot: %+v, %v", record, err)
	}
	pending := mcpRefreshInput(t, ctx, db, record.ID)
	if err := mcpserveraction.NewDeleteMCPServerAction(db).Execute(ctx, identity, record.ID); err != nil {
		t.Fatal(err)
	}
	if err := worker.Execute(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if err := worker.FinalizeFailure(ctx, pending, errors.New("deleted")); err != nil {
		t.Fatal(err)
	}
}

// mcpRefreshInput 读取服务当前批次对应的真实持久化任务。
func mcpRefreshInput(t *testing.T, ctx context.Context, db *bun.DB, serverID string) mcpserveraction.RefreshToolsInput {
	t.Helper()
	var payload string
	err := db.NewRaw(`SELECT tr.payload FROM task_runs tr JOIN mcp_servers ms ON tr.payload->>'refreshId' = ms.tools_refresh_id::text WHERE ms.id = ?`, serverID).Scan(ctx, &payload)
	if err != nil {
		t.Fatal(err)
	}
	var input mcpserveraction.RefreshToolsInput
	if err := json.Unmarshal([]byte(payload), &input); err != nil {
		t.Fatal(err)
	}
	return input
}
