//go:build server

package appservice

import (
	"context"
	"log/slog"

	knowledgebaseaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// ListKnowledgeQAEntries 返回当前企业分组中的问答列表。
func (b *DirectBackend) ListKnowledgeQAEntries(ctx context.Context, meta RequestMeta, knowledgeBaseID string, input KnowledgeQAListInput) (KnowledgeQAList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeQAList{}, err
	}
	output, err := b.listQAEntries.Execute(ctx, identity, knowledgeBaseID, knowledgebaseaction.QAListInput{GroupID: input.GroupID, Keyword: input.Keyword, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		return KnowledgeQAList{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeQAReadFailed, identity.Organization.ID, knowledgeBaseID)
	}
	entries := make([]KnowledgeQASummary, 0, len(output.Entries))
	for _, entry := range output.Entries {
		entries = append(entries, KnowledgeQASummary{ID: entry.ID, GroupID: entry.GroupID, Question: entry.Question,
			SimilarQuestions: entry.SimilarQuestions, Answer: entry.Answer, CreatedAt: entry.CreatedAt})
	}
	return KnowledgeQAList{Entries: entries, Page: PageInfo{Number: output.Page, Size: output.PageSize, Total: output.Total}}, nil
}

// GetKnowledgeQAEntry 返回当前企业的完整问答。
func (b *DirectBackend) GetKnowledgeQAEntry(ctx context.Context, meta RequestMeta, knowledgeBaseID, entryID string) (KnowledgeQAEntry, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeQAEntry{}, err
	}
	record, err := b.getQAEntry.Execute(ctx, identity, knowledgeBaseID, entryID)
	if err != nil {
		return KnowledgeQAEntry{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeQAReadFailed, identity.Organization.ID, knowledgeBaseID)
	}
	return knowledgeQAFromAction(*record), nil
}

// CreateKnowledgeQAEntry 创建本地问答。
func (b *DirectBackend) CreateKnowledgeQAEntry(ctx context.Context, meta RequestMeta, knowledgeBaseID string, input KnowledgeQAInput) (KnowledgeQAEntry, error) {
	return b.saveKnowledgeQAEntry(ctx, meta, knowledgeBaseID, "", input)
}

// UpdateKnowledgeQAEntry 更新本地问答。
func (b *DirectBackend) UpdateKnowledgeQAEntry(ctx context.Context, meta RequestMeta, knowledgeBaseID, entryID string, input KnowledgeQAInput) (KnowledgeQAEntry, error) {
	if !common.ValidUUID(entryID) {
		return KnowledgeQAEntry{}, NotFoundError(meta, cervii18n.ErrorKnowledgeQANotFound)
	}
	return b.saveKnowledgeQAEntry(ctx, meta, knowledgeBaseID, entryID, input)
}

// saveKnowledgeQAEntry 认证当前用户并保存问答内容。
func (b *DirectBackend) saveKnowledgeQAEntry(ctx context.Context, meta RequestMeta, knowledgeBaseID, entryID string, input KnowledgeQAInput) (KnowledgeQAEntry, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeQAEntry{}, err
	}
	questions := make([]knowledgebaseaction.QASimilarQuestion, 0, len(input.SimilarQuestions))
	for _, question := range input.SimilarQuestions {
		questions = append(questions, knowledgebaseaction.QASimilarQuestion{ID: question.ID, Content: question.Content})
	}
	record, err := b.saveQAEntry.Execute(ctx, identity, knowledgeBaseID, entryID, knowledgebaseaction.QAInput{
		GroupID: input.GroupID, Question: input.Question, Answer: input.Answer, SimilarQuestions: questions,
	})
	if err != nil {
		return KnowledgeQAEntry{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeQASaveFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识问答保存成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID, "entry_id", record.ID, "created", entryID == "", "similar_question_count", len(record.SimilarQuestions))
	return knowledgeQAFromAction(*record), nil
}

// DeleteKnowledgeQAEntry 删除当前企业的问答及其内容。
func (b *DirectBackend) DeleteKnowledgeQAEntry(ctx context.Context, meta RequestMeta, knowledgeBaseID, entryID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteQAEntry.Execute(ctx, identity, knowledgeBaseID, entryID); err != nil {
		return b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeQADeleteFailed, identity.Organization.ID, knowledgeBaseID)
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
