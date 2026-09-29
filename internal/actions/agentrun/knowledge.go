//go:build server

package agentrun

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// KnowledgeRetrieval 按企业和知识库范围构造检索来源。
type KnowledgeRetrieval interface {
	Sources(ctx context.Context, organizationID string, knowledgeBaseIDs []string) ([]knowledgeretrieval.Source, error)
}

// loadRunKnowledgeSearch 按本次运行配置版本绑定且仍存在的同企业知识库构造检索函数；没有可用知识库时返回 nil。
func loadRunKnowledgeSearch(ctx context.Context, db bun.IDB, retrieval KnowledgeRetrieval, execution executionContext) (agentruntime.KnowledgeSearch, error) {
	if len(execution.KnowledgeBaseIDs) == 0 {
		return nil, nil
	}
	organizationID := execution.Run.OrganizationID
	var ids []string
	err := db.NewSelect().Model((*servermodels.KnowledgeBase)(nil)).Column("kb.id").
		Where("kb.organization_id = ? AND kb.id IN (?)", organizationID, bun.In(execution.KnowledgeBaseIDs)).
		Order("kb.name").Scan(ctx, &ids)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return func(ctx context.Context, request knowledgeretrieval.Request) (knowledgeretrieval.Result, error) {
		sources, err := retrieval.Sources(ctx, organizationID, ids)
		if err != nil {
			return knowledgeretrieval.Result{}, err
		}
		return knowledgeretrieval.Search(ctx, sources, request)
	}, nil
}
