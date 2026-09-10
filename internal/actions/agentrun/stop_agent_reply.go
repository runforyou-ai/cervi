//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// StopAgentReply 按成员归属停止指定运行，并在提交后尽力中断模型调用。
func (a *ExecuteAction) StopAgentReply(ctx context.Context, identity *servermodels.Identity, conversationID, runID string) (domain.AgentRunStatus, error) {
	if !common.ValidUUID(conversationID) || !common.ValidUUID(runID) {
		return "", chatstate.ErrConversationNotFound
	}
	var status domain.AgentRunStatus
	stopped := false
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		member, err := chatstate.LockMember(ctx, tx, identity, conversationID)
		if err != nil {
			return err
		}
		if member.Conversation.Type != string(domain.ConversationTypeAgent) {
			return chatstate.ErrConversationNotFound
		}
		// 读取权限与新消息发送资格分离，禁用 Agent 后仍可停止自己的运行。
		initial := &servermodels.AgentRun{}
		err = tx.NewSelect().Model(initial).
			Join("JOIN agent_conversations AS ac ON ac.organization_id = agr.organization_id AND ac.conversation_id = agr.conversation_id AND ac.agent_identity_id = agr.agent_identity_id").
			Where("agr.organization_id = ? AND agr.conversation_id = ? AND agr.id = ?", identity.Organization.ID, conversationID, runID).
			Where("ac.user_identity_id = ? AND agr.scope_kind = ?", identity.OrganizationIdentity.ID, domain.AgentExecutionScopeConversation).
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return chatstate.ErrConversationNotFound
		}
		if err != nil {
			return err
		}
		policy := agentChatRunPolicy{enqueuer: a.enqueuer}
		locked, err := lockAgentRun(ctx, tx, policy, initial)
		if err != nil {
			return err
		}
		run, lane := locked.Run, locked.Lane
		status = domain.AgentRunStatus(run.Status)
		if agentRunStatusTerminal(run.Status) {
			return nil
		}
		// 停止边界包含已提交但尚未被模型认领的输入，后到消息另起运行。
		if run.InputStartSeq != lane.ProcessedSeq+1 || lane.DesiredSeq < run.InputStartSeq {
			return errors.New("stopped agent run boundary is inconsistent")
		}
		seqs, err := claimLaneInputs(ctx, tx, run, lane.ProcessedSeq, lane.DesiredSeq)
		if err != nil {
			return err
		}
		if int64(len(seqs)) != lane.DesiredSeq-lane.ProcessedSeq {
			return errors.New("stopped agent input sequence is not contiguous")
		}
		messageID := uuid.NewV7().String()
		if err := policy.persistMessage(ctx, tx, locked.PolicyContext, run, messageID, domain.MessageTypeAgentCancelled, ""); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusCancelled).
			Set("error_code = ?", domain.AgentRunErrorCodeUserCancelled).
			Set("response_message_id = ?", messageID).
			Set("input_end_seq = ?", lane.DesiredSeq).
			Set("completed_at = now()").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(lane).
			Set("processed_seq = ?", lane.DesiredSeq).
			Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return err
		}
		status, stopped = domain.AgentRunStatusCancelled, true
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("stop agent reply: %w", err)
	}
	if status == domain.AgentRunStatusCancelled {
		a.CancelRunContexts([]string{runID})
	}
	if stopped {
		slog.Info("成员停止 AI 回复", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "agent_run_id", runID, "user_id", identity.User.ID)
	}
	return status, nil
}
