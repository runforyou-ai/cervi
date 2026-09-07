//go:build server

package conversation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// loadConversationAgentProcesses 在已授权的消息窗口中批量补充成功过程和最近运行状态。
func loadConversationAgentProcesses(ctx context.Context, db bun.IDB, organizationID, conversationID string, history *ConversationMessageHistory) error {
	var latest struct {
		servermodels.AgentRun `bun:",embed"`
		AgentName             string  `bun:"agent_name"`
		AgentAvatarFileID     *string `bun:"agent_avatar_file_id"`
	}
	err := db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		ColumnExpr("agr.*").ColumnExpr("oi.display_name AS agent_name").
		ColumnExpr("oi.avatar_file_id AS agent_avatar_file_id").
		Join("JOIN organization_identities AS oi ON oi.id = agr.agent_identity_id AND oi.organization_id = agr.organization_id").
		Where("agr.organization_id = ? AND agr.conversation_id = ?", organizationID, conversationID).
		OrderExpr("agr.created_at DESC, agr.id DESC").Limit(1).Scan(ctx, &latest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return fmt.Errorf("load latest conversation agent run: %w", err)
	}
	history.LatestAgentRun = &ConversationAgentRun{ID: latest.ID, AgentName: latest.AgentName, AgentAvatarFileID: latest.AgentAvatarFileID, Status: domain.AgentRunStatus(latest.Status), ErrorCode: latest.ErrorCode, LastError: latest.LastError}
	if len(history.Messages) == 0 {
		return nil
	}
	messagePositions := make(map[string]int, len(history.Messages))
	messageIDs := make([]string, 0, len(history.Messages))
	for i, message := range history.Messages {
		messagePositions[message.ID] = i
		messageIDs = append(messageIDs, message.ID)
	}
	var runs []servermodels.AgentRun
	if err := db.NewSelect().Model(&runs).
		Where("agr.organization_id = ? AND agr.conversation_id = ?", organizationID, conversationID).
		Where("agr.status = ? AND agr.response_message_id IN (?)", domain.AgentRunStatusSucceeded, bun.In(messageIDs)).Scan(ctx); err != nil {
		return fmt.Errorf("load message agent processes: %w", err)
	}
	processes := make(map[string]*ConversationAgentProcess, len(runs))
	runIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		if run.StartedAt == nil || run.CompletedAt == nil || run.ResponseMessageID == nil {
			return fmt.Errorf("load completed agent process: %w", ErrDataInvariant)
		}
		process := &ConversationAgentProcess{ID: run.ID, DurationMilliseconds: run.CompletedAt.Sub(*run.StartedAt).Milliseconds(), Blocks: []agentruntime.Block{}}
		if err := json.Unmarshal(run.Usage, &process.Usage); err != nil {
			return fmt.Errorf("decode agent usage: %w", err)
		}
		history.Messages[messagePositions[*run.ResponseMessageID]].AgentProcess = process
		processes[run.ID] = process
		runIDs = append(runIDs, run.ID)
	}
	if len(runIDs) == 0 {
		return nil
	}
	var blocks []servermodels.AgentRunBlock
	if err := db.NewSelect().Model(&blocks).
		Where("arb.organization_id = ? AND arb.agent_run_id IN (?)", organizationID, bun.In(runIDs)).
		OrderExpr("arb.agent_run_id, arb.position").Scan(ctx); err != nil {
		return fmt.Errorf("load agent process blocks: %w", err)
	}
	for _, block := range blocks {
		var payload agentruntime.BlockPayload
		if err := json.Unmarshal(block.Payload, &payload); err != nil {
			return fmt.Errorf("decode agent process block: %w", err)
		}
		process := processes[block.AgentRunID]
		process.Blocks = append(process.Blocks, agentruntime.Block{ID: block.ID, Position: block.Position, ModelCallID: block.ModelCallID, Kind: domain.AgentRunBlockKind(block.Kind), Payload: payload})
	}
	return nil
}

// loadConversationAgentFailures 按最后消费的消息定位历史错误，并补充增量轮询期间结束的失败运行。
func loadConversationAgentFailures(ctx context.Context, db bun.IDB, organizationID string, input ConversationMessageHistoryInput, history *ConversationMessageHistory) error {
	history.AgentFailures = make([]ConversationAgentFailure, 0)
	messageIDs := make([]string, 0, len(history.Messages))
	for _, message := range history.Messages {
		messageIDs = append(messageIDs, message.ID)
	}
	query := db.NewSelect().TableExpr("agent_runs AS agr").
		ColumnExpr("agr.id, cat.trigger_message_id AS after_message_id, oi.display_name AS agent_name").
		Join("JOIN conversation_agent_triggers AS cat ON cat.organization_id = agr.organization_id AND cat.conversation_id = agr.conversation_id AND cat.agent_identity_id = agr.agent_identity_id AND cat.agent_run_id = agr.id AND cat.trigger_seq = agr.trigger_end_seq").
		Join("JOIN organization_identities AS oi ON oi.organization_id = agr.organization_id AND oi.id = agr.agent_identity_id").
		Where("agr.organization_id = ? AND agr.conversation_id = ? AND agr.status = ?", organizationID, input.ConversationID, domain.AgentRunStatusFailed).
		WhereGroup(" AND ", func(query *bun.SelectQuery) *bun.SelectQuery {
			query = query.Where("cat.trigger_message_id IN (?)", bun.In(messageIDs))
			// 失败不新增聊天消息，空增量页仍需返回上次游标之后结束的运行。
			if input.After != nil {
				query = query.WhereGroup(" OR ", func(query *bun.SelectQuery) *bun.SelectQuery {
					query = query.Where("agr.completed_at >= ?", input.After.OriginatedAt)
					if history.HasLater && history.After != nil {
						query = query.Where("agr.completed_at <= ?", history.After.OriginatedAt)
					}
					return query
				})
			}
			return query
		}).OrderExpr("agr.completed_at, agr.id")
	if err := query.Scan(ctx, &history.AgentFailures); err != nil {
		return fmt.Errorf("load conversation agent failures: %w", err)
	}
	return nil
}
