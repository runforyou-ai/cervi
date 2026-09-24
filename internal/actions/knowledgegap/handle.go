//go:build server

package knowledgegap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// AcceptInput 定义加入知识库的问答：EntryID 为空时在知识库中新建问答，否则更新该问答。
type AcceptInput struct {
	KnowledgeBaseID string
	EntryID         string
	QA              knowledgebase.QAInput
}

// AcceptAction 把待补知识整理的问答加入知识库。
type AcceptAction struct {
	db   *bun.DB
	save *knowledgebase.SaveQAEntryAction
}

// NewAcceptAction 创建加入知识库操作。
func NewAcceptAction(db *bun.DB, save *knowledgebase.SaveQAEntryAction) *AcceptAction {
	return &AcceptAction{db: db, save: save}
}

// Execute 在同一事务中保存问答并把待补知识记为已加入知识库；待处理与已忽略的条目都可以加入。
func (a *AcceptAction) Execute(ctx context.Context, identity *servermodels.Identity, id string, input AcceptInput) error {
	return a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		gap, err := lockGap(ctx, tx, identity.Organization.ID, id)
		if err != nil {
			return err
		}
		if domain.KnowledgeGapStatus(gap.Status) == domain.KnowledgeGapStatusAccepted {
			return ErrHandled
		}
		entry, err := a.save.ExecuteInTx(ctx, tx, identity, input.KnowledgeBaseID, input.EntryID, input.QA)
		if err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(gap).
			Set("status = ?", domain.KnowledgeGapStatusAccepted).
			Set("knowledge_base_id = ?", input.KnowledgeBaseID).
			Set("qa_entry_id = ?", entry.ID).
			Set("handled_by_identity_id = ?", identity.OrganizationIdentity.ID).
			Set("handled_at = now()").
			Set("updated_at = now()").
			WherePK().
			Exec(ctx); err != nil {
			return fmt.Errorf("accept knowledge gap: %w", err)
		}
		return nil
	})
}

// DismissAction 忽略待补知识。
type DismissAction struct{ db *bun.DB }

// NewDismissAction 创建忽略待补知识操作。
func NewDismissAction(db *bun.DB) *DismissAction { return &DismissAction{db: db} }

// Execute 把待处理的条目记为已忽略；已忽略的条目保持不变，已加入知识库的条目不能忽略。
func (a *DismissAction) Execute(ctx context.Context, identity *servermodels.Identity, id string) error {
	return a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		gap, err := lockGap(ctx, tx, identity.Organization.ID, id)
		if err != nil {
			return err
		}
		switch domain.KnowledgeGapStatus(gap.Status) {
		case domain.KnowledgeGapStatusAccepted:
			return ErrHandled
		case domain.KnowledgeGapStatusDismissed:
			return nil
		}
		if _, err := tx.NewUpdate().Model(gap).
			Set("status = ?", domain.KnowledgeGapStatusDismissed).
			Set("handled_by_identity_id = ?", identity.OrganizationIdentity.ID).
			Set("handled_at = now()").
			Set("updated_at = now()").
			WherePK().
			Exec(ctx); err != nil {
			return fmt.Errorf("dismiss knowledge gap: %w", err)
		}
		return nil
	})
}

// lockGap 读取并锁定当前企业的待补知识。
func lockGap(ctx context.Context, tx bun.Tx, organizationID, id string) (*servermodels.KnowledgeGap, error) {
	if !common.ValidUUID(id) {
		return nil, ErrNotFound
	}
	gap := &servermodels.KnowledgeGap{}
	err := tx.NewSelect().Model(gap).Where("kg.organization_id = ? AND kg.id = ?", organizationID, id).For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock knowledge gap: %w", err)
	}
	return gap, nil
}
