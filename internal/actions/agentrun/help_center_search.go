//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/actions/helpcenter"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	"github.com/uptrace/bun"
)

const (
	// helpCenterAnswerTimeout 限制一次帮助中心回答的生成时长，低于服务端 30 秒写超时。
	helpCenterAnswerTimeout = 25 * time.Second
	// helpCenterArticleLimit 是一次搜索返回的相关文章数量上限。
	helpCenterArticleLimit = 5
)

// ErrHelpCenterQueryInvalid 表示帮助中心搜索内容为空或超出长度。
var ErrHelpCenterQueryInvalid = errors.New("help center query is invalid")

// HelpCenterRetrieval 构造只召回帮助中心文章的知识库检索来源。
type HelpCenterRetrieval interface {
	ArticleSources(ctx context.Context, organizationID string, knowledgeBaseIDs []string) ([]knowledgeretrieval.Source, error)
}

// HelpCenterSearchResult 定义帮助中心搜索的 AI 回答与相关文章；首接待不是 AI 员工或资料不足时回答为空。
type HelpCenterSearchResult struct {
	Answer   string
	Articles []helpcenter.ArticleSummary
}

// SearchHelpCenterAction 在网站渠道发布的文章中检索访客问题，首接待为 AI 员工时用其模型单次生成回答，不创建会话或运行记录。
type SearchHelpCenterAction struct {
	db        *bun.DB
	retrieval HelpCenterRetrieval
	generator agentruntime.HelpAnswerGenerator
}

// NewSearchHelpCenterAction 创建帮助中心搜索操作。
func NewSearchHelpCenterAction(db *bun.DB, retrieval HelpCenterRetrieval, generator agentruntime.HelpAnswerGenerator) *SearchHelpCenterAction {
	return &SearchHelpCenterAction{db: db, retrieval: retrieval, generator: generator}
}

// Execute 检索已发布文章并按首接待 AI 员工生成回答。
func (a *SearchHelpCenterAction) Execute(ctx context.Context, channelID, query string) (HelpCenterSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > domain.KnowledgeRetrievalQueryMaxLength {
		return HelpCenterSearchResult{}, ErrHelpCenterQueryInvalid
	}
	scope, err := helpcenter.LoadScope(ctx, a.db, channelID)
	if err != nil {
		return HelpCenterSearchResult{}, err
	}
	result := HelpCenterSearchResult{Articles: make([]helpcenter.ArticleSummary, 0)}
	if len(scope.Bases) == 0 {
		return result, nil
	}
	sources, err := a.retrieval.ArticleSources(ctx, scope.OrganizationID, scope.BaseIDs())
	if err != nil {
		return HelpCenterSearchResult{}, fmt.Errorf("load help center sources: %w", err)
	}
	retrieved, err := knowledgeretrieval.Search(ctx, sources, knowledgeretrieval.Request{Queries: []string{query}})
	if err != nil {
		return HelpCenterSearchResult{}, fmt.Errorf("search help center: %w", err)
	}
	// 按命中顺序去重文章，同时整理回答资料；问答条目以答案作为资料正文。
	materials := make([]agentruntime.HelpAnswerMaterial, 0, len(retrieved.Records))
	seen := map[string]bool{}
	for _, record := range retrieved.Records {
		content := record.Content
		if record.Answer != nil {
			content = *record.Answer
		}
		materials = append(materials, agentruntime.HelpAnswerMaterial{Title: record.DocumentName, Content: content})
		if !seen[record.DocumentID] && len(result.Articles) < helpCenterArticleLimit {
			seen[record.DocumentID] = true
			result.Articles = append(result.Articles, helpcenter.ArticleSummary{ID: record.DocumentID, Title: record.DocumentName})
		}
	}
	if len(materials) == 0 || scope.InitialTargetType != domain.ChannelRoutingTargetTypeMember || scope.InitialTargetID == nil {
		return result, nil
	}
	var agent managedAgentModel
	err = a.db.NewSelect().
		TableExpr("agents AS a").
		Apply(func(query *bun.SelectQuery) *bun.SelectQuery {
			return withManagedAgentConfiguration(query, "a.active_revision_id")
		}).
		Where("a.organization_id = ? AND a.identity_id = ? AND a.status = ?", scope.OrganizationID, *scope.InitialTargetID, domain.UserStatusActive).
		Where("oi.type = ?", domain.OrganizationIdentityTypeAgent).
		Scan(ctx, &agent)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return HelpCenterSearchResult{}, fmt.Errorf("load help center agent configuration: %w", err)
	}
	generateCtx, cancel := context.WithTimeout(ctx, helpCenterAnswerTimeout)
	defer cancel()
	startedAt := time.Now()
	answer, err := a.generator.GenerateHelpAnswer(generateCtx, agentruntime.HelpAnswerRequest{
		Instruction: helpCenterAnswerInstruction(agent.Instruction),
		Model:       agent.modelConfig(),
		Question:    query,
		Materials:   materials,
	})
	attributes := []any{
		"organization_id", scope.OrganizationID, "channel_id", channelID, "agent_identity_id", *scope.InitialTargetID,
		"material_count", len(materials), "duration_ms", time.Since(startedAt).Milliseconds(),
	}
	if err != nil {
		if ctx.Err() != nil {
			return HelpCenterSearchResult{}, ctx.Err()
		}
		// 回答生成失败时仍返回相关文章。
		slog.Warn("帮助中心回答生成失败", append(attributes, "error", err)...)
		return result, nil
	}
	slog.Info("帮助中心回答已生成", append(attributes,
		"answered", answer.Answer != "", "prompt_tokens", answer.Usage.PromptTokens,
		"completion_tokens", answer.Usage.CompletionTokens, "total_tokens", answer.Usage.TotalTokens)...)
	result.Answer = answer.Answer
	return result, nil
}

// helpCenterAnswerInstruction 合并 AI 员工系统指令与帮助中心回答要求。
func helpCenterAnswerInstruction(agentInstruction string) string {
	var instruction strings.Builder
	if text := strings.TrimSpace(agentInstruction); text != "" {
		instruction.WriteString(text)
		instruction.WriteString("\n\n")
	}
	instruction.WriteString("你正在网站帮助中心回答访客搜索的问题，回答直接展示给访客。\n")
	instruction.WriteString("- 只依据提供的帮助文章资料回答，不编造资料中没有的事实、价格、承诺或链接。\n")
	instruction.WriteString("- 资料不足以回答问题时，answer 返回空字符串。\n")
	instruction.WriteString("- 回答简洁直接，可使用 Markdown 列表和强调，不写标题，不提及“资料”“文章”等来源说明。\n")
	instruction.WriteString("- 回答使用的语言与访客问题的语言一致。\n")
	instruction.WriteString(`- 只输出一个 JSON 对象，格式为 {"answer":"回答正文"}，不输出 JSON 以外的任何内容。`)
	return instruction.String()
}
