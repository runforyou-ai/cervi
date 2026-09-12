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
	if err := loadConversationPendingAgents(ctx, db, organizationID, conversationID, history); err != nil {
		return err
	}
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

// loadConversationPendingAgents 按发言顺序读取已收到输入、等待轮转执行的 AI 员工。
func loadConversationPendingAgents(ctx context.Context, db bun.IDB, organizationID, conversationID string, history *ConversationMessageHistory) error {
	rows := make([]ConversationPendingAgent, 0)
	// 队列内输入序号连续，最早一条未处理输入即 processed_seq + 1；执行范围内至多一条活动运行，按其队列排除当前执行者。
	if err := db.NewSelect().TableExpr("agent_lanes AS al").
		ColumnExpr("al.agent_identity_id AS identity_id").
		ColumnExpr("oi.display_name AS display_name").
		ColumnExpr("oi.avatar_file_id::text AS avatar_file_id").
		Join("JOIN organization_identities AS oi ON oi.id = al.agent_identity_id AND oi.organization_id = al.organization_id").
		Join("JOIN agent_inputs AS ai ON ai.lane_id = al.id AND ai.input_seq = al.processed_seq + 1").
		Join("JOIN messages AS msg ON msg.id = ai.source_message_id AND msg.organization_id = ai.organization_id").
		Where("al.organization_id = ?", organizationID).
		Where("al.scope_kind = ? AND al.scope_id = ?", domain.AgentExecutionScopeConversation, conversationID).
		Where("al.desired_seq > al.processed_seq").
		Where(`al.id IS DISTINCT FROM (
			SELECT agr.lane_id FROM agent_runs AS agr
			WHERE agr.organization_id = ? AND agr.scope_kind = ? AND agr.scope_id = ? AND agr.status IN (?, ?)
		)`, organizationID, domain.AgentExecutionScopeConversation, conversationID,
			domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		OrderExpr("msg.message_seq ASC, ai.source_ordinal ASC, al.id ASC").
		Scan(ctx, &rows); err != nil {
		return fmt.Errorf("load conversation pending agents: %w", err)
	}
	history.PendingAgents = rows
	return nil
}
