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
	var latest servermodels.AgentRun
	err := db.NewSelect().Model(&latest).
		Where("agr.organization_id = ? AND agr.conversation_id = ?", organizationID, conversationID).
		OrderExpr("agr.created_at DESC, agr.id DESC").Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return fmt.Errorf("load latest conversation agent run: %w", err)
	}
	history.LatestAgentRun = &ConversationAgentRun{ID: latest.ID, Status: domain.AgentRunStatus(latest.Status), ErrorCode: latest.ErrorCode, LastError: latest.LastError}
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
