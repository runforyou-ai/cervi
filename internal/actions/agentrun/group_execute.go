//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/storage/server/messagequery"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const groupInstructionPreamble = `你是企业 AI 员工「%s」，当前在群聊「%s」中与其他成员一起工作。
群内其他成员的发言以 JSON 提供：sender.name 是发送者名称，sender.kind 为 user 表示真人、为 agent 表示另一位 AI 员工，replyTo 是被引用的原消息；你自己的历史发言是纯文本。
成员点名你或回复你的消息时才轮到你发言。回复直接输出发到群里的正文。`

type groupMentionRunPolicy struct{}

// lockContext 锁定群聊会话并读取执行 Agent 当前的成员关系。
func (p groupMentionRunPolicy) lockContext(ctx context.Context, db bun.IDB, run *servermodels.AgentRun) (agentRunPolicyContext, error) {
	conversation, err := chatstate.LockConversation(ctx, db, run.OrganizationID, run.ConversationID)
	if err != nil {
		return agentRunPolicyContext{}, err
	}
	if conversation.Type != string(domain.ConversationTypeGroup) {
		return agentRunPolicyContext{}, errors.New("agent run does not belong to a group conversation")
	}
	participantID, err := lockGroupAgentParticipant(ctx, db, run.OrganizationID, run.ConversationID, run.AgentIdentityID)
	if err != nil {
		return agentRunPolicyContext{}, err
	}
	return agentRunPolicyContext{Conversation: conversation, AgentParticipantID: participantID}, nil
}

// prepareLocked 校验群仍在使用且执行 Agent 仍是有效成员，失效时收敛本次运行。
func (p groupMentionRunPolicy) prepareLocked(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, run *servermodels.AgentRun) (bool, error) {
	if policyContext.AgentParticipantID != "" &&
		domain.ConversationStatus(policyContext.Conversation.Status) == domain.ConversationStatusActive {
		return true, nil
	}
	if _, err := db.NewUpdate().Model(run).
		Set("status = ?", domain.AgentRunStatusCancelled).
		Set("error_code = ?", domain.AgentRunErrorCodeAgentRemoved).
		Set("last_error = ?", "group agent membership changed").
		Set("completed_at = now()").
		Set("updated_at = now()").
		WherePK().
		Where("status IN (?, ?)", domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Exec(ctx); err != nil {
		return false, fmt.Errorf("suppress group agent run: %w", err)
	}
	slog.Warn("群内 AI 员工失去执行资格，运行已收敛",
		"organization_id", run.OrganizationID, "conversation_id", run.ConversationID,
		"agent_identity_id", run.AgentIdentityID, "agent_run_id", run.ID)
	return false, nil
}

// loadMessages 读取不越过已认领输入的群聊上下文。
func (p groupMentionRunPolicy) loadMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) ([]agentruntime.Message, error) {
	return loadClaimedGroupMessages(ctx, db, run, endSeq)
}

// persistMessage 以 Agent 成员身份追加群聊结果消息。
func (p groupMentionRunPolicy) persistMessage(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, run *servermodels.AgentRun, messageID string, messageType domain.MessageType, content string) error {
	_, _, err := appendAgentMessage(ctx, db, policyContext.Conversation, run, messageID, policyContext.AgentParticipantID, messageType, content, nil)
	return err
}

// instruction 在配置指令前补充本次运行的身份与群聊场景说明。
func (p groupMentionRunPolicy) instruction(ctx context.Context, db bun.IDB, execution executionContext) (string, error) {
	title := ""
	if err := db.NewSelect().Model((*servermodels.Conversation)(nil)).
		ColumnExpr("COALESCE(cv.title, '')").
		Where("cv.organization_id = ? AND cv.id = ?", execution.Run.OrganizationID, execution.Run.ConversationID).
		Scan(ctx, &title); err != nil {
		return "", fmt.Errorf("load group title for instruction: %w", err)
	}
	preamble := fmt.Sprintf(groupInstructionPreamble, execution.AgentName, title)
	if strings.TrimSpace(execution.Instruction) == "" {
		return preamble, nil
	}
	return preamble + "\n\n" + execution.Instruction, nil
}

// laneRevision 在目标 Agent 仍是有效群成员时返回其配置版本。
func (p groupMentionRunPolicy) laneRevision(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, lane *servermodels.AgentLane) (string, bool, error) {
	if domain.ConversationStatus(policyContext.Conversation.Status) != domain.ConversationStatusActive {
		return "", false, nil
	}
	return loadGroupAgentRevision(ctx, db, lane.OrganizationID, lane.ConversationID, lane.AgentIdentityID, false)
}

// lockGroupAgentParticipant 锁定 Agent 在群内的成员关系，已退出时返回空编号。
func lockGroupAgentParticipant(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string) (string, error) {
	var participantID string
	err := db.NewSelect().TableExpr("conversation_participants AS cp").
		ColumnExpr("cp.id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("cp.organization_id = ? AND cp.conversation_id = ?", organizationID, conversationID).
		Where("cs.source_id = ? AND cp.left_at IS NULL", agentIdentityID).
		For("UPDATE OF cp").Scan(ctx, &participantID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("lock group agent participant: %w", err)
	}
	return participantID, nil
}

// loadGroupAgentRevision 读取仍是有效群成员的 Agent 的当前配置版本，requireActive 表示同时要求企业启用状态。
func loadGroupAgentRevision(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string, requireActive bool) (string, bool, error) {
	var revisionID string
	query := db.NewSelect().TableExpr("agents AS a").
		ColumnExpr("a.active_revision_id").
		Join("JOIN agent_revisions AS ar ON ar.id = a.active_revision_id AND ar.agent_id = a.id AND ar.organization_id = a.organization_id AND ar.execution_mode = ? AND ar.schema_version = 1", domain.AgentExecutionModeManaged).
		Join("JOIN chat_subjects AS cs ON cs.organization_id = a.organization_id AND cs.source_id = a.identity_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN conversation_participants AS cp ON cp.organization_id = cs.organization_id AND cp.subject_id = cs.id AND cp.conversation_id = ? AND cp.left_at IS NULL", conversationID).
		Where("a.organization_id = ?", organizationID).
		Where("a.identity_id = ?", agentIdentityID)
	if requireActive {
		query = query.Where("a.status = ?", domain.UserStatusActive)
	}
	err := query.Scan(ctx, &revisionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("load group agent revision: %w", err)
	}
	return revisionID, true, nil
}

type groupMessageRow struct {
	ID               string  `bun:"id"`
	Body             string  `bun:"body"`
	SenderSourceID   string  `bun:"sender_source_id"`
	SenderName       string  `bun:"sender_name"`
	SenderIsAgent    bool    `bun:"sender_is_agent"`
	ReplyToMessageID *string `bun:"reply_to_message_id"`
	ReplyBody        string  `bun:"reply_body"`
	ReplySenderName  string  `bun:"reply_sender_name"`
	ReplyDeleted     bool    `bun:"reply_deleted"`
}

type groupMessageSender struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type groupMessageEnvelope struct {
	Sender  groupMessageSender       `json:"sender"`
	Body    string                   `json:"body"`
	ReplyTo *claimedMessageReference `json:"replyTo,omitempty"`
}

// loadClaimedGroupMessages 读取带发送者标识的群聊上下文，自己的发言投影为助手消息。
func loadClaimedGroupMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) ([]agentruntime.Message, error) {
	boundary, err := loadClaimedMessageBoundary(ctx, db, run, endSeq)
	if err != nil {
		return nil, err
	}
	rows := make([]groupMessageRow, 0, agentHistoryLimit)
	if err := db.NewSelect().TableExpr("messages AS msg").
		ColumnExpr("msg.id, msg.body").
		ColumnExpr("cs.source_id AS sender_source_id").
		ColumnExpr("oi.display_name AS sender_name").
		ColumnExpr("oi.type = ? AS sender_is_agent", domain.OrganizationIdentityTypeAgent).
		ColumnExpr("msg.reply_to_message_id").
		ColumnExpr("? AS reply_body", messagequery.Summary("reply")).
		ColumnExpr("COALESCE(reply_oi.display_name, '') AS reply_sender_name").
		ColumnExpr("reply.deleted_at IS NOT NULL AS reply_deleted").
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN organization_identities AS oi ON oi.id = cs.source_id AND oi.organization_id = cs.organization_id").
		Join("LEFT JOIN messages AS reply ON reply.id = msg.reply_to_message_id AND reply.organization_id = msg.organization_id AND reply.conversation_id = msg.conversation_id AND reply.type IN (?, ?)", domain.MessageTypeText, domain.MessageTypeAttachment).
		Join("LEFT JOIN conversation_participants AS reply_cp ON reply_cp.id = reply.sender_participant_id AND reply_cp.organization_id = reply.organization_id AND reply_cp.conversation_id = reply.conversation_id").
		Join("LEFT JOIN chat_subjects AS reply_cs ON reply_cs.id = reply_cp.subject_id AND reply_cs.organization_id = reply_cp.organization_id AND reply_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("LEFT JOIN organization_identities AS reply_oi ON reply_oi.id = reply_cs.source_id AND reply_oi.organization_id = reply_cs.organization_id").
		Where("msg.organization_id = ?", run.OrganizationID).
		Where("msg.conversation_id = ?", run.ConversationID).
		Where("msg.type = ?", domain.MessageTypeText).
		Where("msg.deleted_at IS NULL").
		Where("msg.message_seq <= ?", boundary.MessageSeq).
		OrderExpr("msg.message_seq DESC").
		Limit(agentHistoryLimit).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load claimed group conversation context: %w", err)
	}
	slices.Reverse(rows)
	messages := make([]agentruntime.Message, 0, len(rows))
	for _, row := range rows {
		// 自己的历史发言保持纯文本，其余成员的发言携带发送者标识与一层引用。
		if row.SenderSourceID == run.AgentIdentityID {
			messages = append(messages, agentruntime.Message{ID: row.ID, Role: agentruntime.MessageRoleAssistant, Content: row.Body})
			continue
		}
		envelope := groupMessageEnvelope{
			Sender: groupMessageSender{Name: row.SenderName, Kind: string(domain.OrganizationIdentityTypeUser)},
			Body:   row.Body,
		}
		if row.SenderIsAgent {
			envelope.Sender.Kind = string(domain.OrganizationIdentityTypeAgent)
		}
		if row.ReplyToMessageID != nil {
			reference := claimedMessageReference{MessageID: *row.ReplyToMessageID, Deleted: row.ReplyDeleted}
			if !row.ReplyDeleted {
				reference.SenderName, reference.Body = row.ReplySenderName, row.ReplyBody
			}
			envelope.ReplyTo = &reference
		}
		encoded, err := json.Marshal(envelope)
		if err != nil {
			return nil, fmt.Errorf("encode group conversation context: %w", err)
		}
		messages = append(messages, agentruntime.Message{ID: row.ID, Role: agentruntime.MessageRoleUser, Content: string(encoded)})
	}
	return messages, nil
}
