//go:build server

package inbox

import (
	"context"
	"database/sql"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ConversationResult 保存指定会话的阅读与列表资格，不可用时不返回摘要。
type ConversationResult struct {
	ID           string
	MatchesQuery bool
	Conversation *ConversationSummary
}

// ReadByIDs 在同一快照中读取指定会话，筛选只决定列表资格，不缩小阅读范围。
func (q *LoadInboxQuery) ReadByIDs(ctx context.Context, identity *servermodels.Identity, ids []string, input *LoadInput) ([]ConversationResult, error) {
	for _, id := range ids {
		if !common.ValidUUID(id) {
			return nil, ErrQueryInvalid
		}
	}
	if input != nil {
		normalized, err := normalizeLoadInput(*input)
		if err != nil {
			return nil, err
		}
		input = &normalized
	}
	results := make([]ConversationResult, 0, len(ids))
	if len(ids) == 0 {
		return results, nil
	}
	err := q.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		snapshot := NewLoadInboxQuery(tx)
		summaries, err := snapshot.readSummaries(ctx, identity, ids)
		if err != nil {
			return err
		}
		matches := make(map[string]bool)
		if input != nil {
			matches, err = snapshot.matchInboxIDs(ctx, identity, ids, *input)
			if err != nil {
				return err
			}
		}
		for _, id := range ids {
			summary := summaries[id]
			results = append(results, ConversationResult{ID: id, Conversation: summary, MatchesQuery: summary != nil && (input == nil || matches[id])})
		}
		return nil
	})
	return results, err
}

// readSummaries 按当前阅读资格批量读取四类会话的公开摘要。
func (q *LoadInboxQuery) readSummaries(ctx context.Context, identity *servermodels.Identity, ids []string) (map[string]*ConversationSummary, error) {
	organizationID, identityID, userID := identity.Organization.ID, identity.OrganizationIdentity.ID, identity.User.ID
	var customers []customerConversationRow
	if err := q.customerConversationDetailsQuery(organizationID, identityID, userID).Where("cv.id IN (?)", bun.In(ids)).Scan(ctx, &customers); err != nil {
		return nil, err
	}
	var directs []directConversationRow
	if err := q.directConversationDetailsQuery(organizationID, identityID, userID).Where("cv.id IN (?)", bun.In(ids)).Scan(ctx, &directs); err != nil {
		return nil, err
	}
	var agents []agentConversationRow
	if err := q.agentConversationDetailsQuery(organizationID, identityID, userID).Where("cv.id IN (?)", bun.In(ids)).Scan(ctx, &agents); err != nil {
		return nil, err
	}
	var groups []groupConversationRow
	if err := q.groupConversationsQuery(organizationID, identityID, userID).Where("cv.id IN (?)", bun.In(ids)).Scan(ctx, &groups); err != nil {
		return nil, err
	}
	summaries := make(map[string]*ConversationSummary, len(ids))
	for _, row := range customers {
		summary := row.summary()
		summaries[row.ID] = &summary
	}
	for _, row := range directs {
		summary := row.summary()
		summaries[row.ID] = &summary
	}
	for _, row := range agents {
		summary := row.summary()
		summaries[row.ID] = &summary
	}
	for _, row := range groups {
		summary := row.summary()
		summaries[row.ID] = &summary
	}
	return summaries, nil
}

// matchInboxIDs 复用列表筛选核对指定 ID，不受分页边界限制。
func (q *LoadInboxQuery) matchInboxIDs(ctx context.Context, identity *servermodels.Identity, ids []string, input LoadInput) (map[string]bool, error) {
	var matched []string
	if err := q.db.NewSelect().TableExpr("(?) AS candidates", q.listCandidates(identity, input)).ColumnExpr("id").Where("id IN (?)", bun.In(ids)).Scan(ctx, &matched); err != nil {
		return nil, err
	}
	matches := make(map[string]bool, len(matched))
	for _, id := range matched {
		matches[id] = true
	}
	return matches, nil
}
