//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// testAgentKnowledgeScopes 验证本地知识库绑定的保存、企业隔离、版本快照和失效解绑。
func testAgentKnowledgeScopes(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string) {
	t.Helper()
	ctx := context.Background()
	bases := make([]string, 0, 2)
	for _, category := range []domain.KnowledgeBaseCategory{domain.KnowledgeBaseCategoryStandard, domain.KnowledgeBaseCategoryQA} {
		base, err := knowledgeaction.NewCreateKnowledgeBaseAction(db).Execute(ctx, identity, newKnowledgeBaseInput(t, db, identity, uuid.NewV7().String(), category))
		if err != nil {
			t.Fatal(err)
		}
		bases = append(bases, base.ID)
	}
	_, foreign := newQAFixture(t, db)
	input := agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{
		ProviderID: providerID, ModelIdentifier: modelID, SystemInstruction: "回答产品问题", KnowledgeBaseIDs: []string{bases[0], bases[0]},
	}}
	create := agentaction.NewCreateAgentAction(db)
	created, err := create.Execute(ctx, identity, agentaction.CreateInput{DisplayName: "本地知识助手", RoleID: roleID, Execution: input})
	if err != nil || !slices.Equal(created.Execution.Managed.KnowledgeBaseIDs, bases[:1]) {
		t.Fatalf("create=%+v err=%v", created, err)
	}
	originalRevisionID := created.Execution.RevisionID
	update := agentaction.NewUpdateExecutionAction(db)
	before, err := db.NewSelect().Model((*servermodels.Agent)(nil)).Where("organization_id = ?", identity.Organization.ID).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 其他企业和不存在的知识库都不能通过创建或编辑写入配置。
	for _, id := range []string{foreign.ID, uuid.NewV7().String()} {
		input.Managed.KnowledgeBaseIDs = []string{id}
		var fields *common.FieldError
		if _, err := update.Execute(ctx, identity, created.ID, agentaction.UpdateExecutionInput{ExecutionInput: input}); !errors.As(err, &fields) || fields.Fields["knowledgeBaseIds"] != agentaction.ValidationKnowledgeBaseInvalid {
			t.Fatalf("invalid update err=%v", err)
		}
		if _, err := create.Execute(ctx, identity, agentaction.CreateInput{DisplayName: "无效绑定助手", RoleID: roleID, Execution: input}); !errors.As(err, &fields) || fields.Fields["knowledgeBaseIds"] != agentaction.ValidationKnowledgeBaseInvalid {
			t.Fatalf("invalid create err=%v", err)
		}
	}
	count, err := db.NewSelect().Model((*servermodels.AgentRevision)(nil)).Where("agent_id = ?", created.ID).Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("invalid updates changed revisions: count=%d err=%v", count, err)
	}
	count, err = db.NewSelect().Model((*servermodels.Agent)(nil)).Where("organization_id = ?", identity.Organization.ID).Count(ctx)
	if err != nil || count != before {
		t.Fatalf("invalid creates left agents: count=%d before=%d err=%v", count, before, err)
	}
	input.Managed.KnowledgeBaseIDs = bases
	updated, err := update.Execute(ctx, identity, created.ID, agentaction.UpdateExecutionInput{ExecutionInput: input})
	if err != nil || updated.Execution.RevisionID == originalRevisionID || !slices.Equal(updated.Execution.Managed.KnowledgeBaseIDs, bases) {
		t.Fatalf("update=%+v err=%v", updated, err)
	}
	// 新配置不能改写旧 Revision 中已经保存的知识库范围。
	var revision servermodels.AgentRevision
	if err := db.NewSelect().Model(&revision).Where("id = ?", originalRevisionID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		KnowledgeBaseIDs []string `json:"knowledgeBaseIds"`
	}
	if err := json.Unmarshal(revision.Configuration, &snapshot); err != nil || !slices.Equal(snapshot.KnowledgeBaseIDs, bases[:1]) {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
	if err := knowledgeaction.NewDeleteKnowledgeBaseAction(db).Execute(ctx, identity, bases[1]); err != nil {
		t.Fatal(err)
	}
	detail, err := agentaction.NewGetAgentQuery(db).Execute(ctx, identity, created.ID)
	if err != nil || !slices.Equal(detail.Execution.Managed.KnowledgeBaseIDs, bases) {
		t.Fatalf("deleted binding detail=%+v err=%v", detail, err)
	}
	if _, err := update.Execute(ctx, identity, created.ID, agentaction.UpdateExecutionInput{ExecutionInput: input}); err == nil {
		t.Fatal("deleted binding saved")
	}
	input.Managed.KnowledgeBaseIDs = []string{}
	cleared, err := update.Execute(ctx, identity, created.ID, agentaction.UpdateExecutionInput{ExecutionInput: input})
	if err != nil || len(cleared.Execution.Managed.KnowledgeBaseIDs) != 0 {
		t.Fatalf("clear=%+v err=%v", cleared, err)
	}
}
