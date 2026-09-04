//go:build server

package agentrun

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
)

type knowledgeSearchFactory interface {
	ForOrganization(context.Context, string) (func(context.Context, knowledgeretrieval.Request) (knowledgeretrieval.Result, error), error)
}

// knowledgeSearchForRun 创建只访问当前企业知识库的检索闭包。
func (a *ExecuteAction) knowledgeSearchForRun(ctx context.Context, organizationID string) (agentruntime.KnowledgeSearch, error) {
	if a.knowledgeSearchFactory == nil {
		return nil, nil
	}
	return a.knowledgeSearchFactory.ForOrganization(ctx, organizationID)
}
