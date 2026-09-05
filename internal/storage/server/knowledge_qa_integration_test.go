//go:build server

package server

import (
	"context"
	"errors"
	"testing"
	"uuid"

	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// newQAFixture 创建独立测试企业和本地问答库。
func newQAFixture(t *testing.T, db *bun.DB) (*servermodels.Identity, *knowledgeaction.Record) {
	t.Helper()
	ctx := context.Background()
	installed, err := installationaction.NewInstallWorkspaceAction(db).Execute(ctx, installationaction.InstallWorkspaceInput{
		AccessHost: uuid.NewV7().String() + ".qa.test", OrganizationName: "问答测试", DisplayName: "维护人员", Email: "owner@qa.test", Password: "password123", Locale: domain.LocaleChineseSimplified, TimeZone: "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	base, err := knowledgeaction.NewCreateKnowledgeBaseAction(db).Execute(ctx, installed.Identity, knowledgeaction.Input{Name: "FAQ", Category: domain.KnowledgeBaseCategoryQA})
	if err != nil {
		t.Fatal(err)
	}
	return installed.Identity, base
}

// TestKnowledgeQALifecycle 验证问答内容编号、分页搜索、分组转移和删除一致性。
func TestKnowledgeQALifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	identity, base := newQAFixture(t, db)
	save := knowledgeaction.NewSaveQAEntryAction(db)
	list := knowledgeaction.NewListQAEntriesQuery(db)
	get := knowledgeaction.NewGetQAEntryQuery(db)
	remove := knowledgeaction.NewDeleteQAEntryAction(db)
	input := knowledgeaction.QAInput{GroupID: base.Groups[0].ID, Question: "  如何退款？  ", Answer: "  进入订单详情。  ", SimilarQuestions: []knowledgeaction.QASimilarQuestion{
		{Content: "退款入口"}, {Content: "退款入口"}, {Content: " 如何退款？ "}, {Content: " "}, {Content: "退还100%费用"},
	}}
	created, err := save.Execute(ctx, identity, base.ID, "", input)
	if err != nil {
		t.Fatal(err)
	}
	if created.Question != "如何退款？" || created.Answer != "进入订单详情。" || len(created.SimilarQuestions) != 2 {
		t.Fatalf("normalized=%+v", created)
	}
	var original []servermodels.KnowledgeQAContent
	if err := db.NewSelect().Model(&original).Where("entry_id = ?", created.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	grouped, err := knowledgeaction.NewCreateKnowledgeGroupAction(db).Execute(ctx, identity, base.ID, knowledgeaction.GroupInput{Name: "售后"})
	if err != nil {
		t.Fatal(err)
	}
	groupID := grouped.Groups[1].ID
	input.GroupID, input.Question, input.Answer = groupID, created.Question, created.Answer
	input.SimilarQuestions = []knowledgeaction.QASimilarQuestion{created.SimilarQuestions[1], created.SimilarQuestions[0]}
	input.SimilarQuestions[1].Content = "退款办理入口"
	updated, err := save.Execute(ctx, identity, base.ID, created.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.GroupID != groupID || updated.SimilarQuestions[0].ID != created.SimilarQuestions[1].ID || updated.SimilarQuestions[1].ID != created.SimilarQuestions[0].ID {
		t.Fatalf("updated=%+v", updated)
	}
	var current []servermodels.KnowledgeQAContent
	if err := db.NewSelect().Model(&current).Where("entry_id = ?", created.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	for _, before := range original {
		found := false
		for _, after := range current {
			if before.ID != after.ID {
				continue
			}
			found = true
			if before.Kind != domain.KnowledgeQAContentSimilarQuestion && !before.UpdatedAt.Equal(after.UpdatedAt) {
				t.Fatal("unchanged content timestamp changed")
			}
		}
		if !found {
			t.Fatalf("content ID replaced: %s", before.ID)
		}
	}
	// 相似问题搜索命中整条问答，百分号按字面匹配，答案不参与问题搜索。
	for keyword, total := range map[string]int{"退款": 1, "100%": 1, "费用": 1, "进入订单": 0, "_": 0} {
		page, err := list.Execute(ctx, identity, base.ID, knowledgeaction.QAListInput{GroupID: groupID, Keyword: keyword, Page: 1, PageSize: 1})
		if err != nil || page.Total != total || len(page.Entries) != total {
			t.Fatalf("query=%q page=%+v err=%v", keyword, page, err)
		}
		if total == 1 {
			entry := page.Entries[0]
			if !entry.CreatedAt.Equal(created.CreatedAt) || entry.Answer != updated.Answer || len(entry.SimilarQuestions) != 2 ||
				entry.SimilarQuestions[0] != updated.SimilarQuestions[0].Content || entry.SimilarQuestions[1] != updated.SimilarQuestions[1].Content {
				t.Fatalf("list content=%+v", entry)
			}
		}
	}
	oldPage, err := list.Execute(ctx, identity, base.ID, knowledgeaction.QAListInput{GroupID: base.Groups[0].ID})
	if err != nil || oldPage.Total != 0 {
		t.Fatalf("old group=%+v err=%v", oldPage, err)
	}
	if _, err := knowledgeaction.NewDeleteKnowledgeGroupAction(db).Execute(ctx, identity, base.ID, groupID); !errors.Is(err, knowledgeaction.ErrGroupNotEmpty) {
		t.Fatalf("delete occupied group=%v", err)
	}
	if _, err := knowledgeaction.NewUpdateKnowledgeBaseAction(db).Execute(ctx, identity, base.ID, knowledgeaction.Input{Name: base.Name, Category: domain.KnowledgeBaseCategoryStandard}); !errors.Is(err, knowledgeaction.ErrBaseHasContent) {
		t.Fatalf("change type=%v", err)
	}
	input.SimilarQuestions = []knowledgeaction.QASimilarQuestion{{Content: "新增相似问题"}}
	updated, err = save.Execute(ctx, identity, base.ID, created.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	exists, err := db.NewSelect().Model((*servermodels.KnowledgeQAContent)(nil)).Where("id = ?", created.SimilarQuestions[0].ID).Exists(ctx)
	if err != nil || exists {
		t.Fatalf("removed content remains: %v", err)
	}
	detail, err := get.Execute(ctx, identity, base.ID, updated.ID)
	if err != nil || detail.SimilarQuestions[0].ID != updated.SimilarQuestions[0].ID {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
	// 添加第二条问答后检查分页总数和不重叠的条目。
	another, err := save.Execute(ctx, identity, base.ID, "", knowledgeaction.QAInput{GroupID: groupID, Question: "另一个问题", Answer: "另一个答案"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := list.Execute(ctx, identity, base.ID, knowledgeaction.QAListInput{GroupID: groupID, Page: 1, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := list.Execute(ctx, identity, base.ID, knowledgeaction.QAListInput{GroupID: groupID, Page: 2, PageSize: 1})
	if err != nil || first.Total != 2 || second.Total != 2 || len(second.Entries) != 1 || first.Entries[0].ID == second.Entries[0].ID {
		t.Fatalf("pages=%+v %+v err=%v", first, second, err)
	}
	if first.Entries[0].ID != another.ID || len(first.Entries[0].SimilarQuestions) != 0 || first.Entries[0].Answer != another.Answer {
		t.Fatalf("list without similar questions=%+v", first)
	}
	if err := remove.Execute(ctx, identity, base.ID, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := get.Execute(ctx, identity, base.ID, created.ID); !errors.Is(err, knowledgeaction.ErrQANotFound) {
		t.Fatalf("deleted entry=%v", err)
	}
	if err := knowledgeaction.NewDeleteKnowledgeBaseAction(db).Execute(ctx, identity, base.ID); err != nil {
		t.Fatal(err)
	}
	count, err := db.NewSelect().Model((*servermodels.KnowledgeQAContent)(nil)).Where("entry_id IN (?, ?)", created.ID, another.ID).Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("orphan contents=%d err=%v", count, err)
	}
	count, err = db.NewSelect().Model((*servermodels.KnowledgeQAEntry)(nil)).Where("knowledge_base_id = ?", base.ID).Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("orphan entries=%d err=%v", count, err)
	}
}

// TestKnowledgeQAIsolation 验证企业、知识库、分组和内容编号边界，以及失败保存的事务回滚。
func TestKnowledgeQAIsolation(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	identity, base := newQAFixture(t, db)
	foreignIdentity, foreignBase := newQAFixture(t, db)
	save := knowledgeaction.NewSaveQAEntryAction(db)
	get := knowledgeaction.NewGetQAEntryQuery(db)
	input := knowledgeaction.QAInput{GroupID: base.Groups[0].ID, Question: "问题", Answer: "答案", SimilarQuestions: []knowledgeaction.QASimilarQuestion{{Content: "相似问题"}}}
	entry, err := save.Execute(ctx, identity, base.ID, "", input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := get.Execute(ctx, foreignIdentity, base.ID, entry.ID); !errors.Is(err, knowledgeaction.ErrNotFound) {
		t.Fatalf("foreign read=%v", err)
	}
	if _, err := save.Execute(ctx, foreignIdentity, base.ID, entry.ID, input); !errors.Is(err, knowledgeaction.ErrNotFound) {
		t.Fatalf("foreign update=%v", err)
	}
	if err := knowledgeaction.NewDeleteQAEntryAction(db).Execute(ctx, foreignIdentity, base.ID, entry.ID); !errors.Is(err, knowledgeaction.ErrNotFound) {
		t.Fatalf("foreign delete=%v", err)
	}
	if _, err := knowledgeaction.NewListQAEntriesQuery(db).Execute(ctx, foreignIdentity, base.ID, knowledgeaction.QAListInput{GroupID: input.GroupID}); !errors.Is(err, knowledgeaction.ErrNotFound) {
		t.Fatalf("foreign list=%v", err)
	}
	input.GroupID = foreignBase.Groups[0].ID
	if _, err := save.Execute(ctx, identity, base.ID, entry.ID, input); !errors.Is(err, knowledgeaction.ErrGroupNotFound) {
		t.Fatalf("foreign group=%v", err)
	}
	sameOrgBase, err := knowledgeaction.NewCreateKnowledgeBaseAction(db).Execute(ctx, identity, knowledgeaction.Input{Name: "其他FAQ", Category: domain.KnowledgeBaseCategoryQA})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := get.Execute(ctx, identity, sameOrgBase.ID, entry.ID); !errors.Is(err, knowledgeaction.ErrQANotFound) {
		t.Fatalf("other base read=%v", err)
	}
	input.GroupID = sameOrgBase.Groups[0].ID
	if _, err := save.Execute(ctx, identity, base.ID, entry.ID, input); !errors.Is(err, knowledgeaction.ErrGroupNotFound) {
		t.Fatalf("other base group=%v", err)
	}
	input.GroupID = base.Groups[0].ID
	input.SimilarQuestions = entry.SimilarQuestions
	// 新条目不得复用已有条目的内容编号，失败后也不留下空条目。
	if _, err := save.Execute(ctx, identity, base.ID, "", input); err == nil {
		t.Fatal("accepted content from another entry")
	}
	count, err := db.NewSelect().Model((*servermodels.KnowledgeQAEntry)(nil)).Where("knowledge_base_id = ?", base.ID).Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("failed insert not rolled back: %d %v", count, err)
	}
	input.Question = "不应保存"
	input.SimilarQuestions = []knowledgeaction.QASimilarQuestion{{ID: uuid.NewV7().String(), Content: "无效编号"}}
	if _, err := save.Execute(ctx, identity, base.ID, entry.ID, input); err == nil {
		t.Fatal("accepted nonexistent content")
	}
	record, err := get.Execute(ctx, identity, base.ID, entry.ID)
	if err != nil || record.Question != entry.Question || !record.UpdatedAt.Equal(entry.UpdatedAt) {
		t.Fatalf("failed update not rolled back: %+v %v", record, err)
	}
	input.Question = " "
	_, err = save.Execute(ctx, identity, base.ID, "", input)
	var fields *common.FieldError
	if !errors.As(err, &fields) || fields.Fields["question"] != knowledgeaction.ValidationQAQuestionRequired {
		t.Fatalf("empty question=%v", err)
	}
	standard, err := knowledgeaction.NewCreateKnowledgeBaseAction(db).Execute(ctx, identity, knowledgeaction.Input{Name: "文档库", Category: domain.KnowledgeBaseCategoryStandard})
	if err != nil {
		t.Fatal(err)
	}
	input.Question, input.GroupID = "问题", standard.Groups[0].ID
	if _, err := save.Execute(ctx, identity, standard.ID, "", input); !errors.Is(err, knowledgeaction.ErrQAUnsupported) {
		t.Fatalf("standard base=%v", err)
	}
}
