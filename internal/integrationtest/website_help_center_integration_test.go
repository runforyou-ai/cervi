//go:build server

package integrationtest

import (
	"context"
	"errors"
	"slices"
	"testing"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	helpcenteraction "github.com/runforyou-ai/cervi/internal/actions/helpcenter"
	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servertest "github.com/runforyou-ai/cervi/internal/servertest"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// testHelpAnswerGenerator 记录帮助中心回答请求并返回固定回答。
type testHelpAnswerGenerator struct {
	requests []agentruntime.HelpAnswerRequest
}

// GenerateHelpAnswer 记录请求并返回固定回答。
func (g *testHelpAnswerGenerator) GenerateHelpAnswer(_ context.Context, request agentruntime.HelpAnswerRequest) (agentruntime.HelpAnswerResult, error) {
	g.requests = append(g.requests, request)
	return agentruntime.HelpAnswerResult{Answer: "签收后七天内可以申请退款。"}, nil
}

// TestWebsiteHelpCenter 验证网站渠道帮助中心的发布范围、合集与文章读取、只检索已发布文章、按首接待生成回答、渠道停用与知识库删除。
func TestWebsiteHelpCenter(t *testing.T) {
	ctx := context.Background()
	store, err := serverstorage.Open(ctx, servertest.DatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	installed, documentBase := newDocumentFixture(t, db)
	identity := installed.Identity
	qaBase, err := knowledgeaction.NewCreateKnowledgeBaseAction(db).Execute(ctx, identity, newKnowledgeBaseInput(t, db, identity, "常见问题", domain.KnowledgeBaseCategoryQA))
	if err != nil {
		t.Fatal(err)
	}
	internalBase, err := knowledgeaction.NewCreateKnowledgeBaseAction(db).Execute(ctx, identity, newKnowledgeBaseInput(t, db, identity, "内部资料", domain.KnowledgeBaseCategoryStandard))
	if err != nil {
		t.Fatal(err)
	}
	probe := &retrievalProbe{}
	text, err := knowledgeaction.NewSaveTextDocumentAction(db, newKnowledgeTasks(t, db)).Execute(ctx, identity, documentBase.ID, "", knowledgeaction.TextDocumentInput{Title: "退款说明", Content: "# 退款\n\n签收后七天内可以申请退款。"})
	if err != nil {
		t.Fatal(err)
	}
	runDocumentProcessing(t, db, probe, identity.Organization.ID, documentBase.ID, text.ID, false)
	fileDocumentID := publishRetrievalDocument(t, db, probe, identity, documentBase, "退款内部流程.txt", "退款审批由财务复核。")
	internalDocumentID := publishRetrievalDocument(t, db, probe, identity, internalBase, "退款底线.txt", "退款最多补偿五十元。")
	qaEntryID := publishQAEntry(t, db, probe, identity, qaBase, knowledgeaction.QAInput{Question: "如何开发票？", Answer: "下单时选择电子发票。"})

	agent, _ := newKnowledgeAgent(t, db, identity, nil)
	channel, err := channelaction.NewCreateMessageChannelAction(db).Execute(ctx, identity, channelaction.CreateMessageChannelInput{
		Type: domain.ChannelTypeWebsite, Name: "帮助中心验证", DefaultLocale: domain.CustomerLocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeMember, ID: agent.IdentityID}, FallbackTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
	})
	if err != nil {
		t.Fatal(err)
	}
	getHelpCenter := helpcenteraction.NewGetHelpCenterQuery(db)
	getArticle := helpcenteraction.NewGetArticleQuery(db)
	generator := &testHelpAnswerGenerator{}
	search := agentrunaction.NewSearchHelpCenterAction(db, knowledgeaction.NewRetrievalService(db, probe, probe), generator)

	// 未发布任何知识库时没有合集，搜索不返回结果。
	if collections, err := getHelpCenter.Execute(ctx, channel.ID); err != nil || len(collections) != 0 {
		t.Fatalf("collections=%+v err=%v", collections, err)
	}

	// 发布范围去重后按知识库名称排序，不属于本企业或格式错误的编号被拒绝。
	update := channelaction.NewUpdateWebsiteChannelHelpCenterAction(db)
	if _, err := update.Execute(ctx, identity, channel.ID, channelaction.WebsiteChannelHelpCenterInput{KnowledgeBaseIDs: []string{"invalid"}}); !errors.As(err, new(*common.FieldError)) {
		t.Fatalf("invalid err=%v", err)
	}
	other, _ := newDocumentFixture(t, db)
	if _, err := update.Execute(ctx, other.Identity, channel.ID, channelaction.WebsiteChannelHelpCenterInput{}); !errors.Is(err, channelaction.ErrNotFound) {
		t.Fatalf("other organization err=%v", err)
	}
	published, err := update.Execute(ctx, identity, channel.ID, channelaction.WebsiteChannelHelpCenterInput{KnowledgeBaseIDs: []string{qaBase.ID, documentBase.ID, qaBase.ID}})
	if err != nil || !slices.Equal(published, []string{qaBase.ID, documentBase.ID}) {
		t.Fatalf("published=%v err=%v", published, err)
	}
	detail, err := channelaction.NewGetWebsiteChannelQuery(db).Execute(ctx, identity, channel.ID)
	if err != nil || !slices.Equal(detail.HelpCenterKnowledgeBaseIDs, published) {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}

	// 合集只包含在线编写的文档与问答条目。
	collections, err := getHelpCenter.Execute(ctx, channel.ID)
	if err != nil || len(collections) != 2 {
		t.Fatalf("collections=%+v err=%v", collections, err)
	}
	if collections[0].ID != qaBase.ID || len(collections[0].Articles) != 1 || collections[0].Articles[0] != (helpcenteraction.ArticleSummary{ID: qaEntryID, Title: "如何开发票？"}) {
		t.Fatalf("qa collection=%+v", collections[0])
	}
	if collections[1].ID != documentBase.ID || len(collections[1].Articles) != 1 || collections[1].Articles[0] != (helpcenteraction.ArticleSummary{ID: text.ID, Title: "退款说明"}) {
		t.Fatalf("document collection=%+v", collections[1])
	}

	// 文章详情返回正文，文件文档与未发布知识库的文档不可读取。
	article, err := getArticle.Execute(ctx, channel.ID, text.ID)
	if err != nil || article.Body != "# 退款\n\n签收后七天内可以申请退款。" || article.CollectionName != documentBase.Name {
		t.Fatalf("article=%+v err=%v", article, err)
	}
	article, err = getArticle.Execute(ctx, channel.ID, qaEntryID)
	if err != nil || article.Title != "如何开发票？" || article.Body != "下单时选择电子发票。" {
		t.Fatalf("qa article=%+v err=%v", article, err)
	}
	for _, id := range []string{fileDocumentID, internalDocumentID, "invalid"} {
		if _, err := getArticle.Execute(ctx, channel.ID, id); !errors.Is(err, helpcenteraction.ErrArticleNotFound) {
			t.Fatalf("article %s err=%v", id, err)
		}
	}

	// 搜索只召回已发布文章，首接待 AI 员工据此生成回答。
	result, err := search.Execute(ctx, channel.ID, "退款")
	if err != nil || result.Answer != "签收后七天内可以申请退款。" || len(result.Articles) == 0 || result.Articles[0].ID != text.ID {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, article := range result.Articles {
		if article.ID == fileDocumentID || article.ID == internalDocumentID {
			t.Fatalf("unpublished article=%+v", article)
		}
	}
	if len(generator.requests) != 1 || generator.requests[0].Question != "退款" {
		t.Fatalf("requests=%+v", generator.requests)
	}
	for _, material := range generator.requests[0].Materials {
		if material.Title != "退款说明" && material.Title != "如何开发票？" {
			t.Fatalf("unpublished material=%+v", material)
		}
	}
	if _, err := search.Execute(ctx, channel.ID, "  "); !errors.Is(err, agentrunaction.ErrHelpCenterQueryInvalid) {
		t.Fatalf("empty query err=%v", err)
	}

	// 首接待不是 AI 员工时只返回相关文章。
	if _, err := db.NewUpdate().Model((*servermodels.Channel)(nil)).Set("initial_routing_target_type = ?", domain.ChannelRoutingTargetTypePublicQueue).Set("initial_routing_target_id = NULL").Where("id = ?", channel.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	result, err = search.Execute(ctx, channel.ID, "退款")
	if err != nil || result.Answer != "" || len(result.Articles) == 0 || len(generator.requests) != 1 {
		t.Fatalf("queue result=%+v err=%v requests=%d", result, err, len(generator.requests))
	}

	// 删除知识库后从帮助中心移除。
	if err := knowledgeaction.NewDeleteKnowledgeBaseAction(db).Execute(ctx, identity, qaBase.ID); err != nil {
		t.Fatal(err)
	}
	collections, err = getHelpCenter.Execute(ctx, channel.ID)
	if err != nil || len(collections) != 1 || collections[0].ID != documentBase.ID {
		t.Fatalf("after delete collections=%+v err=%v", collections, err)
	}

	// 渠道停用后帮助中心不可访问。
	if _, err := db.NewUpdate().Model((*servermodels.Channel)(nil)).Set("enabled = FALSE").Where("id = ?", channel.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := getHelpCenter.Execute(ctx, channel.ID); !errors.Is(err, helpcenteraction.ErrChannelNotFound) {
		t.Fatalf("disabled err=%v", err)
	}
	if _, err := search.Execute(ctx, channel.ID, "退款"); !errors.Is(err, helpcenteraction.ErrChannelNotFound) {
		t.Fatalf("disabled search err=%v", err)
	}
}
