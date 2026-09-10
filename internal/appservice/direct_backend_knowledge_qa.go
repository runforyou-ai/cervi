//go:build server

package appservice

import (
	"context"
	"log/slog"

	knowledgebaseaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ListKnowledgeQAEntries 返回当前企业分组中的问答列表。
func (o *directOperations) ListKnowledgeQAEntries(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID string, input KnowledgeQAListInput) (KnowledgeQAList, error) {
	output, err := o.listQAEntries.Execute(ctx, identity, knowledgeBaseID, knowledgebaseaction.QAListInput{GroupID: input.GroupID, Keyword: input.Keyword, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		return KnowledgeQAList{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeQAReadFailed, identity.Organization.ID, knowledgeBaseID)
	}
	entries := make([]KnowledgeQASummary, 0, len(output.Entries))
	for _, entry := range output.Entries {
		entries = append(entries, KnowledgeQASummary{ID: entry.ID, GroupID: entry.GroupID, Question: entry.Question,
			SimilarQuestions: entry.SimilarQuestions, Answer: entry.Answer, CreatedAt: entry.CreatedAt})
	}
	return KnowledgeQAList{Entries: entries, Page: PageInfo{Number: output.Page, Size: output.PageSize, Total: output.Total}}, nil
}

// GetKnowledgeQAEntry 返回当前企业的完整问答。
func (o *directOperations) GetKnowledgeQAEntry(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID, entryID string) (KnowledgeQAEntry, error) {
	record, err := o.getQAEntry.Execute(ctx, identity, knowledgeBaseID, entryID)
	if err != nil {
		return KnowledgeQAEntry{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeQAReadFailed, identity.Organization.ID, knowledgeBaseID)
	}
	return knowledgeQAFromAction(*record), nil
}

// CreateKnowledgeQAEntry 创建本地问答。
func (o *directOperations) CreateKnowledgeQAEntry(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID string, input KnowledgeQAInput) (KnowledgeQAEntry, error) {
	return o.saveKnowledgeQAEntry(ctx, meta, identity, knowledgeBaseID, "", input)
}

// UpdateKnowledgeQAEntry 更新本地问答。
func (o *directOperations) UpdateKnowledgeQAEntry(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID, entryID string, input KnowledgeQAInput) (KnowledgeQAEntry, error) {
	if !common.ValidUUID(entryID) {
		return KnowledgeQAEntry{}, NotFoundError(meta, cervii18n.ErrorKnowledgeQANotFound)
	}
	return o.saveKnowledgeQAEntry(ctx, meta, identity, knowledgeBaseID, entryID, input)
}

// saveKnowledgeQAEntry 保存问答内容。
func (o *directOperations) saveKnowledgeQAEntry(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID, entryID string, input KnowledgeQAInput) (KnowledgeQAEntry, error) {
	questions := make([]knowledgebaseaction.QASimilarQuestion, 0, len(input.SimilarQuestions))
	for _, question := range input.SimilarQuestions {
		questions = append(questions, knowledgebaseaction.QASimilarQuestion{ID: question.ID, Content: question.Content})
	}
	record, err := o.saveQAEntry.Execute(ctx, identity, knowledgeBaseID, entryID, knowledgebaseaction.QAInput{
		GroupID: input.GroupID, Question: input.Question, Answer: input.Answer, SimilarQuestions: questions,
	})
	if err != nil {
		return KnowledgeQAEntry{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeQASaveFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识问答保存成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID, "entry_id", record.ID, "created", entryID == "", "similar_question_count", len(record.SimilarQuestions))
	return knowledgeQAFromAction(*record), nil
}

// DeleteKnowledgeQAEntry 删除当前企业的问答及其内容。
func (o *directOperations) DeleteKnowledgeQAEntry(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID, entryID string) error {
	if err := o.deleteQAEntry.Execute(ctx, identity, knowledgeBaseID, entryID); err != nil {
		return o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeQADeleteFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识问答删除成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID, "entry_id", entryID)
	return nil
}

// knowledgeQAFromAction 转换问答详情并保持相似问题顺序。
func knowledgeQAFromAction(record knowledgebaseaction.QARecord) KnowledgeQAEntry {
	questions := make([]KnowledgeQASimilarQuestion, 0, len(record.SimilarQuestions))
	for _, question := range record.SimilarQuestions {
		questions = append(questions, KnowledgeQASimilarQuestion{ID: question.ID, Content: question.Content})
	}
	return KnowledgeQAEntry{ID: record.ID, GroupID: record.GroupID, Question: record.Question, Answer: record.Answer,
		SimilarQuestions: questions, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}
