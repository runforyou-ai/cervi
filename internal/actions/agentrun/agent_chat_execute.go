//go:build server

package agentrun

import (
	"context"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type agentChatRunPolicy struct{}

// lockContext 锁定 AI 会话及其固定 Agent 的有效参与关系。
func (p agentChatRunPolicy) lockContext(ctx context.Context, db bun.IDB, run *servermodels.AgentRun) (agentRunPolicyContext, error) {
	cv, err := chatstate.LockConversation(ctx, db, run.OrganizationID, run.ConversationID)
	if err != nil {
		return agentRunPolicyContext{}, err
	}
	if cv.Type != string(domain.ConversationTypeAgent) {
		return agentRunPolicyContext{}, errors.New("agent run does not belong to an AI conversation")
	}
	// 校验已提交输入的固定归属和发送关系。
	var participantID string
	if err := db.NewSelect().TableExpr("conversation_participants AS cp").
		ColumnExpr("cp.id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Join("JOIN agent_conversations AS ac ON ac.conversation_id = cp.conversation_id AND ac.organization_id = cp.organization_id AND ac.agent_identity_id = cs.source_id").
		Where("cp.organization_id = ? AND cp.conversation_id = ?", run.OrganizationID, run.ConversationID).
		Where("cp.left_at IS NULL AND cs.kind = ? AND ac.agent_identity_id = ?", domain.ChatSubjectKindOrganizationIdentity, run.AgentIdentityID).
		For("UPDATE OF cp").Scan(ctx, &participantID); err != nil {
		return agentRunPolicyContext{}, fmt.Errorf("lock AI conversation agent participant: %w", err)
	}
	return agentRunPolicyContext{Conversation: cv, AgentParticipantID: participantID}, nil
}

// prepareLocked 确认 AI 聊天 Agent 可以继续执行。
func (p agentChatRunPolicy) prepareLocked(context.Context, bun.IDB, agentRunPolicyContext, *servermodels.AgentRun) (bool, error) {
	return true, nil
}

// loadMessages 按会话稳定顺序读取 AI 聊天 Agent 上下文。
func (p agentChatRunPolicy) loadMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) ([]agentruntime.Message, error) {
	return loadClaimedConversationMessages(ctx, db, run, endSeq)
}

// persistMessage 追加独立 AI 会话的结果消息。
func (p agentChatRunPolicy) persistMessage(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, run *servermodels.AgentRun, messageID string, messageType domain.MessageType, content string) error {
	_, _, err := appendAgentMessage(ctx, db, policyContext.Conversation, run, messageID, policyContext.AgentParticipantID, messageType, content, nil)
	return err
}

// instruction 沿用 AI 会话配置的系统提示词。
func (p agentChatRunPolicy) instruction(_ context.Context, _ bun.IDB, execution executionContext) (string, error) {
	return execution.Instruction, nil
}

// laneRevision 读取 AI 聊天 Agent 当前生效的配置版本。
func (p agentChatRunPolicy) laneRevision(ctx context.Context, db bun.IDB, _ agentRunPolicyContext, lane *servermodels.AgentLane) (string, bool, error) {
	var revisionID string
	if err := db.NewSelect().Model((*servermodels.Agent)(nil)).
		Column("active_revision_id").
		Where("a.identity_id = ?", lane.AgentIdentityID).
		Where("a.organization_id = ?", lane.OrganizationID).
		Scan(ctx, &revisionID); err != nil {
		return "", false, fmt.Errorf("load next agent run revision: %w", err)
	}
	return revisionID, true, nil
}
