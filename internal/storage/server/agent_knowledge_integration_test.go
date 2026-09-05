//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/common"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/connector"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type testKnowledgeBackend struct{}

// Get 返回测试知识库的检索配置。
func (testKnowledgeBackend) Get(context.Context, connector.DifyKnowledgeBaseConfig, string) (connector.DifyKnowledgeBaseDetail, error) {
	return connector.DifyKnowledgeBaseDetail{RetrievalModelJSON: json.RawMessage(`{}`)}, nil
}

// Retrieve 让每个实际访问的数据集返回可辨认的片段。
func (testKnowledgeBackend) Retrieve(_ context.Context, _ connector.DifyKnowledgeBaseConfig, dataset, _ string, _ json.RawMessage) ([]connector.DifyKnowledgeRetrievalRecord, error) {
	return []connector.DifyKnowledgeRetrievalRecord{{DocumentID: "document", SegmentID: dataset, Position: 1, Content: dataset}}, nil
}

// testAgentKnowledgeScopes 验证两个入口的版本冻结、企业隔离和失效绑定恢复。
func testAgentKnowledgeScopes(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string) {
	t.Helper()
	ctx := context.Background()
	connection := &servermodels.IntegrationConnection{
		OrganizationID: identity.Organization.ID, Type: string(domain.IntegrationConnectionTypeDify),
		Name: "Agent 知识范围测试", Configuration: servermodels.IntegrationConnectionConfiguration{APIURL: "https://example.com", APIKey: "test"},
	}
	if _, err := db.NewInsert().Model(connection).Column("organization_id", "connector_type", "name", "configuration").Returning("id").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	bases := make([]servermodels.KnowledgeBase, 3)
	for i := range bases {
		dataset := uuid.NewV7().String()
		organizationID := identity.Organization.ID
		if i == 2 {
			organizationID = uuid.NewV7().String()
		}
		bases[i] = servermodels.KnowledgeBase{
			OrganizationID: organizationID, CreatedByUserID: identity.User.ID, Name: dataset,
			Category: "standard", IntegrationConnectionID: &connection.ID, ExternalResourceID: &dataset,
		}
		if _, err := db.NewInsert().Model(&bases[i]).Column("organization_id", "created_by_user_id", "name", "category", "integration_connection_id", "external_resource_id").Returning("id").Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}
	searchService := knowledgeaction.NewSearchService(db, testKnowledgeBackend{}, testKnowledgeBackend{}, nil)
	// 组织范围必须同时约束绑定加载和游标读取。
	if _, err := searchService.ForKnowledgeBases(ctx, identity.Organization.ID, []string{bases[2].ID}); err == nil {
		t.Fatal("foreign scope accepted")
	}
	search, err := searchService.ForKnowledgeBases(ctx, identity.Organization.ID, []string{bases[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{bases[1].ID, bases[2].ID} {
		if _, err := search(ctx, knowledgeretrieval.Request{Cursor: &knowledgeretrieval.Cursor{KnowledgeBaseID: id, DocumentID: "doc", SegmentID: "segment", Position: 1}}); err == nil {
			t.Fatal("out-of-scope cursor accepted")
		}
	}
	tasks := servertask.New(db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	scheduler := agentrunaction.NewScheduler(tasks)
	for _, website := range []bool{false, true} {
		name := "单聊"
		if website {
			name = "网站客服"
		}
		t.Run(name, func(t *testing.T) {
			testAgentKnowledgeRuns(t, db, identity, roleID, providerID, modelID, bases, tasks, scheduler, searchService, website)
		})
	}
}

// testAgentKnowledgeRuns 验证一个会话入口在切换配置与删除知识库后的运行行为。
func testAgentKnowledgeRuns(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string, bases []servermodels.KnowledgeBase, tasks *servertask.Runtime, scheduler *agentrunaction.Scheduler, searchService *knowledgeaction.SearchService, website bool) {
	t.Helper()
	ctx := context.Background()
	input := agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{
		ProviderID: providerID, ModelIdentifier: modelID, SystemInstruction: "回答问题", KnowledgeBaseIDs: []string{bases[0].ID},
	}}
	created, err := agentaction.NewCreateAgentAction(db).Execute(ctx, identity, agentaction.CreateInput{DisplayName: "知识助手", RoleID: roleID, Execution: input})
	if err != nil {
		t.Fatal(err)
	}
	originalRevisionID := created.Execution.RevisionID
	update := agentaction.NewUpdateExecutionAction(db)
	// 非法更新不产生新版本，创建也使用相同的事务校验。
	input.Managed.KnowledgeBaseIDs = []string{bases[2].ID}
	_, err = update.Execute(ctx, identity, created.ID, input)
	var fields *common.FieldError
	if !errors.As(err, &fields) || fields.Fields["knowledgeBaseIds"] != agentaction.ValidationKnowledgeBaseInvalid {
		t.Fatalf("foreign binding error = %v", err)
	}
	_, err = agentaction.NewCreateAgentAction(db).Execute(ctx, identity, agentaction.CreateInput{DisplayName: "非法知识助手", RoleID: roleID, Execution: input})
	if !errors.As(err, &fields) || fields.Fields["knowledgeBaseIds"] != agentaction.ValidationKnowledgeBaseInvalid {
		t.Fatalf("foreign create error = %v", err)
	}
	var conversationID, channelID string
	if website {
		channel, err := channelaction.NewCreateMessageChannelAction(db).Execute(ctx, identity, channelaction.CreateMessageChannelInput{
			Type: domain.ChannelTypeWebsite, Name: "知识测试网站", DefaultLocale: domain.LocaleChineseSimplified,
			NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeMember, ID: created.IdentityID},
			FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		})
		if err != nil {
			t.Fatal(err)
		}
		channelID = channel.ID
	}
	// 通过真实消息入口创建每个 Run。
	send := func() *servermodels.AgentRun {
		t.Helper()
		if website {
			var previous *string
			if conversationID != "" {
				previous = &conversationID
			}
			output, err := conversationaction.NewReceiveWebsiteCustomerTextMessageAction(db, scheduler).Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
				ChannelID: channelID, ExternalID: "web-session:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ConversationID: previous, ClientMessageID: uuid.NewV7().String(), Body: "查询知识",
			})
			if err != nil {
				t.Fatal(err)
			}
			conversationID = output.Conversation.ID
		} else if conversationID == "" {
			output, err := conversationaction.NewSendFirstDirectTextMessageAction(db, scheduler).Execute(ctx, identity, conversationaction.FirstDirectTextMessageInput{
				TargetIdentityID: created.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "查询知识",
			})
			if err != nil {
				t.Fatal(err)
			}
			conversationID = output.Conversation.ID
		} else {
			if _, err := conversationaction.NewSendDirectTextMessageAction(db, scheduler).Execute(ctx, identity, conversationaction.DirectTextMessageInput{
				ConversationID: conversationID, ClientMessageID: uuid.NewV7().String(), Body: "查询知识",
			}); err != nil {
				t.Fatal(err)
			}
		}
		run := &servermodels.AgentRun{}
		if err := db.NewSelect().Model(run).Where("agr.conversation_id = ?", conversationID).Where("agr.status = ?", domain.AgentRunStatusQueued).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		return run
	}
	expected := []string{bases[0].ID}
	runtimeCalls := 0
	runtime := testAgentRuntime{run: func(ctx context.Context, request agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		runtimeCalls++
		if len(expected) == 0 {
			if request.KnowledgeSearch != nil {
				t.Fatal("empty scope registered knowledge tool")
			}
		} else {
			if request.KnowledgeSearch == nil {
				t.Fatal("missing knowledge tool")
			}
			result, err := request.KnowledgeSearch(ctx, knowledgeretrieval.Request{Queries: []string{"查询一", "查询二"}})
			if err != nil {
				return agentruntime.RunResult{}, err
			}
			ids := make([]string, 0, len(result.Records))
			for _, record := range result.Records {
				ids = append(ids, record.KnowledgeBaseID)
			}
			slices.Sort(ids)
			want := slices.Clone(expected)
			slices.Sort(want)
			if !slices.Equal(ids, want) {
				t.Fatalf("actual knowledge sources = %v, want %v", ids, want)
			}
		}
		triggers, err := feed.Peek(ctx, 0)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		claimed, err := feed.Claim(ctx, triggers[len(triggers)-1].Seq)
		return agentruntime.RunResult{Content: "完成", EndSeq: claimed.EndSeq}, err
	}}
	execute := agentrunaction.NewExecuteAction(db, tasks, runtime, searchService)
	oldRun := send()
	input.Managed.KnowledgeBaseIDs = []string{bases[0].ID, bases[1].ID}
	updated, err := update.Execute(ctx, identity, created.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if oldRun.AgentRevisionID != originalRevisionID || updated.Execution.RevisionID == originalRevisionID {
		t.Fatal("revision was not frozen")
	}
	if err := execute.Execute(ctx, agentrunaction.RunInput{RunID: oldRun.ID}); err != nil {
		t.Fatal(err)
	}
	newRun := send()
	expected = input.Managed.KnowledgeBaseIDs
	if newRun.AgentRevisionID != updated.Execution.RevisionID {
		t.Fatal("new run did not use updated revision")
	}
	if err := execute.Execute(ctx, agentrunaction.RunInput{RunID: newRun.ID}); err != nil {
		t.Fatal(err)
	}
	// 删除后新运行必须失败，详情仍保留绑定，用户可以清空配置恢复。
	if _, err := db.NewDelete().Model(&bases[1]).WherePK().Exec(ctx); err != nil {
		t.Fatal(err)
	}
	// 恢复测试数据供另一个会话入口独立验证。
	t.Cleanup(func() {
		if _, err := db.NewInsert().Model(&bases[1]).Exec(ctx); err != nil {
			t.Error(err)
		}
	})
	invalidRun := send()
	beforeCalls := runtimeCalls
	if err := execute.Execute(ctx, agentrunaction.RunInput{RunID: invalidRun.ID}); err == nil {
		t.Fatal("invalid scope run succeeded")
	}
	if err := db.NewSelect().Model(invalidRun).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if runtimeCalls != beforeCalls || invalidRun.Status != string(domain.AgentRunStatusFailed) || invalidRun.LastError == nil || !strings.Contains(*invalidRun.LastError, "knowledge") {
		t.Fatalf("invalid scope run = %#v", invalidRun)
	}
	// 在恢复前验证失败运行已经消费其输入边界。
	state := &servermodels.ConversationAgentState{}
	if err := db.NewSelect().Model(state).Where("cas.conversation_id = ?", conversationID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if invalidRun.TriggerEndSeq == nil || state.ProcessedSeq != *invalidRun.TriggerEndSeq || state.ProcessedSeq != state.DesiredSeq {
		t.Fatalf("scope failure did not advance state: run=%#v state=%#v", invalidRun, state)
	}
	detail, err := agentaction.NewGetAgentQuery(db).Execute(ctx, identity, created.ID)
	if err != nil || len(detail.Execution.Managed.KnowledgeBaseIDs) != 2 {
		t.Fatalf("invalid binding detail = %#v, error = %v", detail, err)
	}
	if _, err := update.Execute(ctx, identity, created.ID, input); err == nil {
		t.Fatal("deleted binding saved")
	}
	input.Managed.KnowledgeBaseIDs = []string{}
	if _, err := update.Execute(ctx, identity, created.ID, input); err != nil {
		t.Fatal(err)
	}
	expected = nil
	recoveredRun := send()
	if err := execute.Execute(ctx, agentrunaction.RunInput{RunID: recoveredRun.ID}); err != nil {
		t.Fatal(err)
	}
	if err := db.NewSelect().Model(state).Where("cas.conversation_id = ?", conversationID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if state.ProcessedSeq != state.DesiredSeq {
		t.Fatalf("scope failure stalled state: %#v", state)
	}
}
