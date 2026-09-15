//go:build server

package integrationtest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	aiprovideraction "github.com/runforyou-ai/cervi/internal/actions/aiprovider"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	servertest "github.com/runforyou-ai/cervi/internal/servertest"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
)

type testKnowledgeRuntime struct {
	t     *testing.T
	check func(agentruntime.KnowledgeSearch)
}

// Run 认领全部输入后交给当前阶段的检查函数验证知识库检索工具。
func (r *testKnowledgeRuntime) Run(ctx context.Context, request agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
	triggers, err := feed.Peek(ctx, 0)
	if err != nil {
		return agentruntime.RunResult{}, err
	}
	claimed, err := feed.Claim(ctx, triggers[len(triggers)-1].Seq)
	if err != nil {
		return agentruntime.RunResult{}, err
	}
	if request.KnowledgeSearch == nil {
		r.t.Fatal("bound agent did not receive knowledge search")
	}
	r.check(request.KnowledgeSearch)
	return agentruntime.RunResult{Content: "已查阅资料", EndSeq: claimed.EndSeq}, nil
}

// TestAgentKnowledgeSearch 验证运行期按配置版本绑定的知识库注入检索、跨库融合、游标阅读、范围隔离以及知识库删除后的行为。
func TestAgentKnowledgeSearch(t *testing.T) {
	ctx := context.Background()
	store, err := serverstorage.Open(ctx, servertest.DatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	installed, refundBase := newDocumentFixture(t, db)
	identity := installed.Identity
	invoiceBase, err := knowledgeaction.NewCreateKnowledgeBaseAction(db).Execute(ctx, identity, newKnowledgeBaseInput(t, db, identity, "发票", domain.KnowledgeBaseCategoryStandard))
	if err != nil {
		t.Fatal(err)
	}
	unboundBase, err := knowledgeaction.NewCreateKnowledgeBaseAction(db).Execute(ctx, identity, newKnowledgeBaseInput(t, db, identity, "配送", domain.KnowledgeBaseCategoryStandard))
	if err != nil {
		t.Fatal(err)
	}
	probe := &retrievalProbe{}
	refundID := publishRetrievalDocument(t, db, probe, identity, refundBase, "退款政策.txt", strings.Repeat("签收后七天内可以申请退款，退款金额原路返回。", 30))
	invoiceID := publishRetrievalDocument(t, db, probe, identity, invoiceBase, "发票说明.txt", "下单时可以选择开具电子发票。")
	publishRetrievalDocument(t, db, probe, identity, unboundBase, "配送说明.txt", "配送时效按收货地址计算。")

	provider, err := aiprovideraction.NewCreateAIProviderAction(db).Execute(ctx, identity, aiprovideraction.Input{
		Brand: domain.AIProviderBrandOpenAI, Name: uuid.NewV7().String(), APIKey: "test-key", APIURL: "https://models.test/v1",
		Models: []aiprovideraction.Model{{Identifier: "chat", Name: "对话模型", Type: domain.AIModelTypeChat, InputModalities: []domain.AIModelInputModality{domain.AIModelInputModalityText}, ContextWindow: 32000, MaxOutputTokens: 4096}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var roleID string
	if err := db.NewSelect().Table("roles").Column("id").Where("organization_id = ? AND kind = ?", identity.Organization.ID, domain.RoleKindCustomerService).Scan(ctx, &roleID); err != nil {
		t.Fatal(err)
	}
	agent, err := agentaction.NewCreateAgentAction(db).Execute(ctx, identity, agentaction.CreateInput{
		DisplayName: "资料助手", RoleID: roleID,
		Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{
			ProviderID: provider.ID, ModelIdentifier: "chat", SystemInstruction: "依据知识库回答", KnowledgeBaseIDs: []string{refundBase.ID, invoiceBase.ID},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tasks := servertask.New(db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	scheduler := agentrunaction.NewScheduler(tasks)
	first, err := conversationaction.NewSendFirstAgentTextMessageAction(db, scheduler).Execute(ctx, identity, conversationaction.FirstAgentTextMessageInput{
		ConversationID: uuid.NewV7().String(), AgentIdentityID: agent.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "怎么退款和开发票",
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &testKnowledgeRuntime{t: t}
	execute := agentrunaction.NewExecuteAction(db, tasks, runtime, testAttachmentReader(db), knowledgeaction.NewRetrievalService(db, probe, probe))
	// runQueued 同步执行会话中排队的运行并确认成功收尾。
	runQueued := func() {
		t.Helper()
		run := &servermodels.AgentRun{}
		if err := db.NewSelect().Model(run).Where("agr.conversation_id = ? AND agr.status = ?", first.Conversation.ID, domain.AgentRunStatusQueued).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if err := execute.Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
			t.Fatal(err)
		}
		if err := db.NewSelect().Model(run).WherePK().Scan(ctx); err != nil || domain.AgentRunStatus(run.Status) != domain.AgentRunStatusSucceeded {
			t.Fatalf("run=%+v err=%v", run, err)
		}
	}

	// 两个绑定库的结果经统一融合返回，未绑定库不进入范围；命中记录可按游标读取相邻分段。
	runtime.check = func(search agentruntime.KnowledgeSearch) {
		result, err := search(ctx, knowledgeretrieval.Request{Queries: []string{"退款", "发票", "配送"}})
		if err != nil || len(result.Records) == 0 {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		bases := map[string]string{}
		for _, record := range result.Records {
			if record.KnowledgeBaseID == unboundBase.ID || record.Cursor.SegmentID == "" || !record.Matched || record.Score == nil {
				t.Fatalf("record=%+v", record)
			}
			bases[record.KnowledgeBaseID] = record.KnowledgeBaseName
		}
		if bases[refundBase.ID] != refundBase.Name || bases[invoiceBase.ID] != invoiceBase.Name || len(bases) != 2 {
			t.Fatalf("bases=%v", bases)
		}
		var cursor knowledgeretrieval.Cursor
		for _, record := range result.Records {
			if record.DocumentID == refundID && record.Position == 1 {
				cursor = record.Cursor
			}
		}
		window, err := search(ctx, knowledgeretrieval.Request{Cursor: &cursor, After: 1})
		if err != nil || len(window.Records) != 2 || window.Records[0].SegmentID != cursor.SegmentID || window.Records[1].Position != 2 || window.Records[0].Matched {
			t.Fatalf("window=%+v err=%v", window, err)
		}
		// 阅读结果的游标可以继续读取，且携带知识库标识。
		next := window.Records[1]
		if next.KnowledgeBaseID != refundBase.ID || next.KnowledgeBaseName != refundBase.Name || next.Cursor.SegmentID != next.SegmentID || next.Cursor.Position != 2 {
			t.Fatalf("next=%+v", next)
		}
		if more, err := search(ctx, knowledgeretrieval.Request{Cursor: &next.Cursor, Before: 1}); err != nil || len(more.Records) != 2 || more.Records[0].Position != 1 || more.Records[1].SegmentID != next.SegmentID {
			t.Fatalf("more=%+v err=%v", more, err)
		}
		foreign := knowledgeretrieval.Cursor{KnowledgeBaseID: unboundBase.ID, DocumentID: refundID, SegmentID: cursor.SegmentID, Position: 1}
		if _, err := search(ctx, knowledgeretrieval.Request{Cursor: &foreign}); err == nil {
			t.Fatal("unbound knowledge base cursor readable")
		}
	}
	runQueued()

	// 删除一个绑定库后，运行继续在剩余库中检索，结果不再包含已删除库的文档。
	if err := knowledgeaction.NewDeleteKnowledgeBaseAction(db).Execute(ctx, identity, invoiceBase.ID); err != nil {
		t.Fatal(err)
	}
	send := conversationaction.NewSendAgentTextMessageAction(db, scheduler)
	if _, err := send.Execute(ctx, identity, conversationaction.InternalTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "再问一次发票"}); err != nil {
		t.Fatal(err)
	}
	runtime.check = func(search agentruntime.KnowledgeSearch) {
		result, err := search(ctx, knowledgeretrieval.Request{Queries: []string{"退款", "发票"}})
		if err != nil || len(result.Records) == 0 || result.Records[0].DocumentID != refundID {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		for _, record := range result.Records {
			if record.KnowledgeBaseID != refundBase.ID || record.DocumentID == invoiceID {
				t.Fatalf("record=%+v", record)
			}
		}
	}
	runQueued()

	// 绑定库全部删除后，检索向模型明确报告范围失效。
	if err := knowledgeaction.NewDeleteKnowledgeBaseAction(db).Execute(ctx, identity, refundBase.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := send.Execute(ctx, identity, conversationaction.InternalTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "还有资料吗"}); err != nil {
		t.Fatal(err)
	}
	runtime.check = func(search agentruntime.KnowledgeSearch) {
		if _, err := search(ctx, knowledgeretrieval.Request{Queries: []string{"退款"}}); !errors.Is(err, agentrunaction.ErrKnowledgeScopeDeleted) {
			t.Fatalf("err=%v", err)
		}
	}
	runQueued()
	history, err := conversationaction.NewListConversationMessagesQuery(db).Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: first.Conversation.ID})
	if err != nil || len(history.Messages) != 6 {
		t.Fatalf("messages=%d err=%v", len(history.Messages), err)
	}
}
