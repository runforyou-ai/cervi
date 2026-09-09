//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	mcpaction "github.com/runforyou-ai/cervi/internal/actions/mcpserver"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// newAgentMCPService 创建无法连接且没有工具的服务，验证绑定无需远端可用性。
func newAgentMCPService(t *testing.T, db *bun.DB, identity *servermodels.Identity) string {
	t.Helper()
	service := &servermodels.MCPServer{
		OrganizationID: identity.Organization.ID, Name: uuid.NewV7().String(),
		URL: "http://127.0.0.1:1/mcp", ServerType: domain.MCPServerTypeStreamableHTTP, ToolsFailure: "unavailable",
	}
	if _, err := db.NewInsert().Model(service).Column("organization_id", "name", "url", "server_type", "tools_failure").Returning("id").Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	return service.ID
}

// testAgentMCPServices 验证整体保存、企业隔离、删除联动和并发事务。
func testAgentMCPServices(t *testing.T, db *bun.DB, owner *servermodels.Identity, roleID, providerID, modelID string) {
	t.Helper()
	ctx := context.Background()
	execution := agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{
		ProviderID: providerID, ModelIdentifier: modelID, SystemInstruction: "回答产品问题",
	}}
	create := agentaction.NewCreateAgentAction(db)
	update := agentaction.NewUpdateExecutionAction(db)
	get := agentaction.NewGetAgentQuery(db)
	remove := mcpaction.NewDeleteMCPServerAction(db)
	agents := make([]*agentaction.Agent, 0, 2)
	for range 2 {
		agent, err := create.Execute(ctx, owner, agentaction.CreateInput{DisplayName: "MCP 配置助手", RoleID: roleID, Execution: execution})
		if err != nil {
			t.Fatal(err)
		}
		if len(agent.Execution.MCPServerIDs) != 0 {
			t.Fatal("new agent has MCP bindings")
		}
		agents = append(agents, agent)
	}
	ids := []string{newAgentMCPService(t, db, owner), newAgentMCPService(t, db, owner)}
	slices.Sort(ids)
	foreignOwner, _ := newQAFixture(t, db)
	foreignID := newAgentMCPService(t, db, foreignOwner)
	options, err := agentaction.NewListMCPServerOptionsQuery(db).Execute(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if !slices.ContainsFunc(options, func(o agentaction.MCPServerOption) bool { return o.ID == id && o.ToolCount == 0 }) {
			t.Fatalf("missing offline service %s: %+v", id, options)
		}
	}
	if slices.ContainsFunc(options, func(o agentaction.MCPServerOption) bool { return o.ID == foreignID }) {
		t.Fatal("foreign service leaked")
	}
	for _, agent := range agents {
		saved, err := update.Execute(ctx, owner, agent.ID, agentaction.UpdateExecutionInput{ExecutionInput: execution, MCPServerIDs: []string{ids[1], ids[0], ids[0]}})
		if err != nil || !slices.Equal(saved.Execution.MCPServerIDs, ids) {
			t.Fatalf("save=%+v err=%v", saved, err)
		}
	}
	var before servermodels.AgentRevision
	if err := db.NewSelect().Model(&before).Where("agent_id = ?", agents[0].ID).OrderExpr("created_at DESC, id DESC").Limit(1).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal(before.Configuration, &snapshot); err != nil {
		t.Fatal(err)
	}
	if _, exists := snapshot["mcpServerIds"]; exists {
		t.Fatal("current MCP bindings copied into immutable revision")
	}
	for _, id := range []string{foreignID, uuid.NewV7().String(), "invalid"} {
		_, err := update.Execute(ctx, owner, agents[0].ID, agentaction.UpdateExecutionInput{ExecutionInput: execution, MCPServerIDs: []string{id}})
		var fields *common.FieldError
		if !errors.As(err, &fields) || fields.Fields["mcpServerIds"] != agentaction.ValidationMCPServerInvalid {
			t.Fatalf("invalid binding: %v", err)
		}
	}
	// 关系更新后的其他字段校验失败，也必须回滚绑定和版本。
	invalid := agentaction.ExecutionInput{Mode: execution.Mode, Managed: &agentaction.ManagedExecutionInput{ProviderID: providerID, ModelIdentifier: "missing-model", SystemInstruction: "新指令"}}
	if _, err := update.Execute(ctx, owner, agents[0].ID, agentaction.UpdateExecutionInput{ExecutionInput: invalid}); err == nil {
		t.Fatal("invalid model saved")
	}
	detail, err := get.Execute(ctx, owner, agents[0].ID)
	if err != nil || detail.Execution.RevisionID != before.ID || !slices.Equal(detail.Execution.MCPServerIDs, ids) {
		t.Fatalf("rollback detail=%+v err=%v", detail, err)
	}
	if err := remove.Execute(ctx, foreignOwner, ids[0]); !errors.Is(err, mcpaction.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	if err := remove.Execute(ctx, owner, ids[0]); err != nil {
		t.Fatal(err)
	}
	for _, agent := range agents {
		detail, err := get.Execute(ctx, owner, agent.ID)
		if err != nil || !slices.Equal(detail.Execution.MCPServerIDs, ids[1:]) {
			t.Fatalf("deleted binding detail=%+v err=%v", detail, err)
		}
	}
	var after servermodels.AgentRevision
	if err := db.NewSelect().Model(&after).Where("id = ?", before.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if string(before.Configuration) != string(after.Configuration) {
		t.Fatal("deletion changed revision")
	}
	cleared, err := update.Execute(ctx, owner, agents[0].ID, agentaction.UpdateExecutionInput{ExecutionInput: execution})
	if err != nil || len(cleared.Execution.MCPServerIDs) != 0 {
		t.Fatalf("clear=%+v err=%v", cleared, err)
	}

	// 使用两个真实用户和查询屏障，避免用户行锁掩盖服务及员工行锁。
	colleague := newChatLockUser(t, db, owner)
	db.AddQueryHook(chatQueryHook{})
	for _, saveFirst := range []bool{true, false} {
		name := "删除先取得锁"
		if saveFirst {
			name = "保存先取得锁"
		}
		t.Run(name, func(t *testing.T) {
			testAgentMCPDeleteRace(t, db, owner, colleague, agents[0].ID, execution, saveFirst)
		})
	}
	t.Run("两次保存串行替换", func(t *testing.T) {
		testAgentMCPSaveRace(t, db, owner, colleague, agents[0].ID, execution)
	})
}

// testAgentMCPDeleteRace 验证服务删除与绑定保存按取锁顺序收敛。
func testAgentMCPDeleteRace(t *testing.T, db *bun.DB, owner, colleague *servermodels.Identity, agentID string, execution agentaction.ExecutionInput, saveFirst bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id := newAgentMCPService(t, db, owner)
	gate := newChatQueryGate(t, false, 1, func(e *bun.QueryEvent) bool {
		return strings.Contains(e.Query, `FROM "mcp_servers"`) && strings.Contains(e.Query, id) && strings.Contains(e.Query, "FOR ")
	})
	// 失败路径也要先释放事务，再交由外层连接清理。
	defer gate.open()
	gated := context.WithValue(ctx, chatQueryGateKey{}, gate)
	update := agentaction.NewUpdateExecutionAction(db)
	remove := mcpaction.NewDeleteMCPServerAction(db)
	saved, deleted := make(chan error, 1), make(chan error, 1)
	saveCtx, deleteCtx := ctx, gated
	if saveFirst {
		saveCtx, deleteCtx = gated, ctx
	}
	save := func() {
		_, err := update.Execute(saveCtx, owner, agentID, agentaction.UpdateExecutionInput{ExecutionInput: execution, MCPServerIDs: []string{id}})
		saved <- err
	}
	deletion := func() { deleted <- remove.Execute(deleteCtx, colleague, id) }
	if saveFirst {
		go save()
	} else {
		go deletion()
	}
	waitChatSignal(t, ctx, gate.reached)
	if saveFirst {
		go deletion()
	} else {
		go save()
	}
	waitChatDatabaseLock(t, ctx, db, `FROM "mcp_servers"`, id)
	gate.open()
	saveErr := waitChatResult(t, ctx, saved)
	if saveFirst && saveErr != nil {
		t.Fatal(saveErr)
	}
	if !saveFirst {
		var fields *common.FieldError
		if !errors.As(saveErr, &fields) || fields.Fields["mcpServerIds"] != agentaction.ValidationMCPServerInvalid {
			t.Fatalf("stale save: %v", saveErr)
		}
	}
	if err := waitChatResult(t, ctx, deleted); err != nil {
		t.Fatal(err)
	}
	count, err := db.NewSelect().Model((*servermodels.AgentMCPServer)(nil)).Where("mcp_server_id = ?", id).Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("dangling bindings=%d err=%v", count, err)
	}
}

// testAgentMCPSaveRace 验证同一员工两次保存不会把两个草稿合并。
func testAgentMCPSaveRace(t *testing.T, db *bun.DB, owner, colleague *servermodels.Identity, agentID string, execution agentaction.ExecutionInput) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ids := []string{newAgentMCPService(t, db, owner), newAgentMCPService(t, db, owner)}
	gate := newChatQueryGate(t, false, 1, func(e *bun.QueryEvent) bool {
		return strings.Contains(e.Query, `FROM "agents"`) && strings.Contains(e.Query, agentID) && strings.Contains(e.Query, "FOR UPDATE")
	})
	defer gate.open()
	update := agentaction.NewUpdateExecutionAction(db)
	first, second := make(chan error, 1), make(chan error, 1)
	go func() {
		_, err := update.Execute(context.WithValue(ctx, chatQueryGateKey{}, gate), owner, agentID, agentaction.UpdateExecutionInput{ExecutionInput: execution, MCPServerIDs: ids[:1]})
		first <- err
	}()
	waitChatSignal(t, ctx, gate.reached)
	go func() {
		_, err := update.Execute(ctx, colleague, agentID, agentaction.UpdateExecutionInput{ExecutionInput: execution, MCPServerIDs: ids[1:]})
		second <- err
	}()
	waitChatDatabaseLock(t, ctx, db, `FROM "agents"`, agentID)
	gate.open()
	if err := waitChatResult(t, ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := waitChatResult(t, ctx, second); err != nil {
		t.Fatal(err)
	}
	detail, err := agentaction.NewGetAgentQuery(db).Execute(ctx, owner, agentID)
	if err != nil || !slices.Equal(detail.Execution.MCPServerIDs, ids[1:]) {
		t.Fatalf("concurrent save=%+v err=%v", detail, err)
	}
}
