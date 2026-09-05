//go:build server

package knowledgebase

import (
	"context"
	"strings"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListQAEntriesQuery 查询本地问答分组中的内容。
type ListQAEntriesQuery struct{ db *bun.DB }

// NewListQAEntriesQuery 创建问答列表查询。
func NewListQAEntriesQuery(db *bun.DB) *ListQAEntriesQuery { return &ListQAEntriesQuery{db: db} }

// Execute 按分组及主问题或相似问题返回稳定分页的问答列表。
func (q *ListQAEntriesQuery) Execute(ctx context.Context, identity *servermodels.Identity, knowledgeBaseID string, input QAListInput) (QAListOutput, error) {
	base, err := loadKnowledgeBase(ctx, q.db, identity.Organization.ID, knowledgeBaseID)
	if err != nil {
		return QAListOutput{}, err
	}
	if err := validateQAKnowledgeBase(base); err != nil {
		return QAListOutput{}, err
	}
	if _, err := loadKnowledgeGroup(ctx, q.db, identity.Organization.ID, knowledgeBaseID, input.GroupID); err != nil {
		return QAListOutput{}, err
	}
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 || input.PageSize > 100 {
		input.PageSize = 20
	}
	records := make([]QASummary, 0)
	query := q.db.NewSelect().TableExpr("knowledge_qa_entries AS kqe").
		ColumnExpr("kqe.id, kqe.group_id, kqe.created_at, primary_content.content AS question, answer_content.content AS answer").
		ColumnExpr("ARRAY(SELECT similar_content.content FROM knowledge_qa_contents AS similar_content WHERE similar_content.entry_id = kqe.id AND similar_content.kind = ? ORDER BY similar_content.sort_order, similar_content.id) AS similar_questions", domain.KnowledgeQAContentSimilarQuestion).
		Join("JOIN knowledge_qa_contents AS primary_content ON primary_content.entry_id = kqe.id AND primary_content.kind = ?", domain.KnowledgeQAContentPrimaryQuestion).
		Join("JOIN knowledge_qa_contents AS answer_content ON answer_content.entry_id = kqe.id AND answer_content.kind = ?", domain.KnowledgeQAContentAnswer).
		Where("kqe.knowledge_base_id = ?", knowledgeBaseID).Where("kqe.group_id = ?", input.GroupID)
	if keyword := strings.TrimSpace(input.Keyword); keyword != "" {
		// 按字面匹配问题文本，避免通配符改变用户输入的含义。
		pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(keyword) + "%"
		query = query.Where("EXISTS (SELECT 1 FROM knowledge_qa_contents AS matched WHERE matched.entry_id = kqe.id AND matched.kind IN (?, ?) AND matched.content ILIKE ?)", domain.KnowledgeQAContentPrimaryQuestion, domain.KnowledgeQAContentSimilarQuestion, pattern)
	}
	total, err := query.OrderExpr("kqe.updated_at DESC, kqe.id DESC").Limit(input.PageSize).
		Offset((input.Page-1)*input.PageSize).ScanAndCount(ctx, &records)
	return QAListOutput{Entries: records, Page: input.Page, PageSize: input.PageSize, Total: total}, err
}
