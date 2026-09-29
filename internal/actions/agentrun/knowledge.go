//go:build server

package agentrun

import (
	"context"
	"errors"
	"log/slog"

	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// KnowledgeRetrieval 按企业和知识库范围构造检索来源。
type KnowledgeRetrieval interface {
	Sources(ctx context.Context, organizationID string, knowledgeBaseIDs []string) ([]knowledgeretrieval.Source, error)
}

// ErrKnowledgeScopeDeleted 表示本次运行绑定的知识库已全部删除。
var ErrKnowledgeScopeDeleted = errors.New("knowledge bases bound to this run have been deleted")

// loadKnowledgeSearch 按配置版本绑定且仍存在的同企业知识库构造检索函数；没有绑定时返回 nil。
func loadKnowledgeSearch(ctx context.Context, db bun.IDB, retrieval KnowledgeRetrieval, organizationID string, knowledgeBaseIDs []string) (agentruntime.KnowledgeSearch, error) {
	if len(knowledgeBaseIDs) == 0 {
		return nil, nil
	}
	var ids []string
	err := db.NewSelect().Model((*servermodels.KnowledgeBase)(nil)).Column("kb.id").
		Where("kb.organization_id = ? AND kb.id IN (?)", organizationID, bun.In(knowledgeBaseIDs)).
		Order("kb.name").Scan(ctx, &ids)
	if err != nil {
		return nil, err
	}
	if len(ids) < len(knowledgeBaseIDs) {
		slog.Warn("Agent 配置绑定的知识库已有缺失", "organization_id", organizationID,
			"bound_count", len(knowledgeBaseIDs), "available_count", len(ids))
	}
	return func(ctx context.Context, request knowledgeretrieval.Request) (knowledgeretrieval.Result, error) {
		if len(ids) == 0 {
			return knowledgeretrieval.Result{}, ErrKnowledgeScopeDeleted
		}
		sources, err := retrieval.Sources(ctx, organizationID, ids)
		if err != nil {
			return knowledgeretrieval.Result{}, err
		}
		return knowledgeretrieval.Search(ctx, sources, request)
	}, nil
}
