//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/storage/server/messagequery"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const groupSceneRules = `本次在群聊「%s」中与其他成员一起工作。
群内其他成员的发言以 JSON 提供：sender.name 是发送者名称，sender.kind 为 user 表示真人、为 agent 表示另一位 AI 员工，mentions 是这条消息点名的成员，replyTo 是被引用的原消息，attachment 是消息携带的附件；你自己的历史发言是纯文本。
addressedToYou 为 true 的消息是本次需要你处理的请求，其余消息是群内上下文。
你的最终回复会原样发到群里。需要某位成员回应时，在正文中写「@成员名」：@ 前留空格（位于行首时除外），成员名后接空格或标点；被点名的 AI 员工会接着发言。可点名的成员：%s。`

// markdownParser 按 CommonMark 语法解析回复正文，用于识别代码范围。
var markdownParser = goldmark.DefaultParser()

type groupMentionRunPolicy struct {
	scheduler *Scheduler
}

// lockContext 锁定群聊会话并读取执行 Agent 当前的成员关系。
func (p groupMentionRunPolicy) lockContext(ctx context.Context, db bun.IDB, run *servermodels.AgentRun) (agentRunPolicyContext, error) {
	conversation, err := chatstate.LockConversation(ctx, db, run.OrganizationID, run.ConversationID)
	if err != nil {
		return agentRunPolicyContext{}, err
	}
	if conversation.Type != string(domain.ConversationTypeGroup) {
		return agentRunPolicyContext{}, errors.New("agent run does not belong to a group conversation")
	}
	participantID, subjectID, err := lockGroupAgentParticipant(ctx, db, run.OrganizationID, run.ConversationID, run.AgentIdentityID)
	if err != nil {
		return agentRunPolicyContext{}, err
	}
	return agentRunPolicyContext{Conversation: conversation, AgentParticipantID: participantID, AgentSubjectID: subjectID}, nil
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
func (p groupMentionRunPolicy) loadMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64, links attachmentLinks) ([]agentruntime.Message, error) {
	return loadClaimedGroupMessages(ctx, db, run, endSeq, links)
}

// persistMessage 以 Agent 成员身份追加群聊结果消息。
func (p groupMentionRunPolicy) persistMessage(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, run *servermodels.AgentRun, messageID string, messageType domain.MessageType, content string) error {
	_, _, err := appendAgentMessage(ctx, db, policyContext.Conversation, run, messageID, policyContext.AgentParticipantID, messageType, content, nil)
	return err
}

// sceneRules 给出群聊场景、消息格式、可点名成员与工具用法。
func (p groupMentionRunPolicy) sceneRules(ctx context.Context, db bun.IDB, execution executionContext, tools behaviorTools) (agentruntime.Scene, string, error) {
	title := ""
	if err := db.NewSelect().Model((*servermodels.Conversation)(nil)).
		ColumnExpr("COALESCE(cv.title, '')").
		Where("cv.organization_id = ? AND cv.id = ?", execution.Run.OrganizationID, execution.Run.ConversationID).
		Scan(ctx, &title); err != nil {
		return "", "", fmt.Errorf("load group title for scene rules: %w", err)
	}
	participants, err := loadGroupMentionParticipants(ctx, db, execution.Run.OrganizationID, execution.Run.ConversationID, execution.Run.AgentIdentityID)
	if err != nil {
		return "", "", err
	}
	// 按名称排序列出群内名称唯一的可点名成员，没有可点名成员时明确告知。
	names := make([]string, 0, len(participants))
	for name, matched := range participants {
		if len(matched) == 1 {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	candidates := "无"
	if len(names) > 0 {
		candidates = strings.Join(names, "、")
	}
	return agentruntime.SceneGroup, composeInstruction(fmt.Sprintf(groupSceneRules, title, candidates), toolGuidance(tools), ""), nil
}

// laneRevision 在目标 Agent 仍是有效群成员时返回其配置版本。
func (p groupMentionRunPolicy) laneRevision(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, lane *servermodels.AgentLane) (string, bool, error) {
	if domain.ConversationStatus(policyContext.Conversation.Status) != domain.ConversationStatusActive {
		return "", false, nil
	}
	return loadGroupAgentRevision(ctx, db, lane.OrganizationID, lane.ConversationID, lane.AgentIdentityID, false)
}

// lockGroupAgentParticipant 锁定 Agent 在群内的成员关系，已退出时返回空编号。
func lockGroupAgentParticipant(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string) (string, string, error) {
	var row struct {
		ParticipantID string `bun:"participant_id"`
		SubjectID     string `bun:"subject_id"`
	}
	err := db.NewSelect().TableExpr("conversation_participants AS cp").
		ColumnExpr("cp.id AS participant_id").
		ColumnExpr("cs.id AS subject_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("cp.organization_id = ? AND cp.conversation_id = ?", organizationID, conversationID).
		Where("cs.source_id = ? AND cp.left_at IS NULL", agentIdentityID).
		For("UPDATE OF cp").Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("lock group agent participant: %w", err)
	}
	return row.ParticipantID, row.SubjectID, nil
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
	MentionAll       bool    `bun:"mention_all"`
	ReplyToMessageID *string `bun:"reply_to_message_id"`
	ReplyBody        string  `bun:"reply_body"`
	ReplySenderName  string  `bun:"reply_sender_name"`
	ReplyDeleted     bool    `bun:"reply_deleted"`
	contextAttachmentRow
}

type groupMessageSender struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type groupMessageEnvelope struct {
	Sender         groupMessageSender       `json:"sender"`
	Body           string                   `json:"body"`
	AddressedToYou bool                     `json:"addressedToYou,omitempty"`
	Mentions       []groupMessageSender     `json:"mentions,omitempty"`
	MentionAll     bool                     `json:"mentionAll,omitempty"`
	Attachment     *contextAttachment       `json:"attachment,omitempty"`
	ReplyTo        *claimedMessageReference `json:"replyTo,omitempty"`
}

// loadClaimedGroupMessages 读取带发送者标识的群聊上下文，自己的发言投影为助手消息。
func loadClaimedGroupMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64, links attachmentLinks) ([]agentruntime.Message, error) {
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
		ColumnExpr("msg.mention_all").
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
		Apply(withContextAttachments).
		Where("msg.organization_id = ?", run.OrganizationID).
		Where("msg.conversation_id = ?", run.ConversationID).
		Where("msg.deleted_at IS NULL").
		Where("msg.message_seq <= ?", boundary.MessageSeq).
		OrderExpr("msg.message_seq DESC").
		Limit(agentHistoryLimit).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load claimed group conversation context: %w", err)
	}
	slices.Reverse(rows)
	messageIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		messageIDs = append(messageIDs, row.ID)
	}
	mentions, err := loadGroupMessageMentions(ctx, db, run.OrganizationID, messageIDs)
	if err != nil {
		return nil, err
	}
	addressed, err := loadClaimedInputMessages(ctx, db, run, endSeq)
	if err != nil {
		return nil, err
	}
	messages := make([]agentruntime.Message, 0, len(rows))
	for _, row := range rows {
		// 自己的历史发言保持纯文本，其余成员的发言携带发送者标识与一层引用。
		if row.SenderSourceID == run.AgentIdentityID {
			messages = append(messages, agentruntime.Message{ID: row.ID, Role: agentruntime.MessageRoleAssistant, Content: row.Body})
			continue
		}
		envelope := groupMessageEnvelope{
			Sender:         groupMessageSender{Name: row.SenderName, Kind: string(domain.OrganizationIdentityTypeUser)},
			Body:           row.Body,
			Mentions:       mentions[row.ID],
			MentionAll:     row.MentionAll,
			AddressedToYou: addressed[row.ID],
			Attachment:     row.attachment(row.ID, links),
		}
		if row.SenderIsAgent {
			envelope.Sender.Kind = string(domain.OrganizationIdentityTypeAgent)
		}
		if row.ReplyToMessageID != nil {
			reference := claimedMessageReference{MessageID: *row.ReplyToMessageID, Deleted: row.ReplyDeleted}
			if !row.ReplyDeleted {
				reference.SenderName, reference.Body = row.ReplySenderName, row.ReplyBody
				reference.Attachment = row.replyAttachment(links)
			}
			envelope.ReplyTo = &reference
		}
		encoded, err := json.Marshal(envelope)
		if err != nil {
			return nil, fmt.Errorf("encode group conversation context: %w", err)
		}
		messages = append(messages, agentruntime.Message{ID: row.ID, Role: agentruntime.MessageRoleUser, Content: string(encoded), Media: row.media()})
	}
	return messages, nil
}

// loadGroupMessageMentions 按消息读取被点名成员，供上下文说明本轮参与者。
func loadGroupMessageMentions(ctx context.Context, db bun.IDB, organizationID string, messageIDs []string) (map[string][]groupMessageSender, error) {
	mentions := make(map[string][]groupMessageSender, len(messageIDs))
	if len(messageIDs) == 0 {
		return mentions, nil
	}
	rows := make([]struct {
		MessageID    string `bun:"message_id"`
		DisplayName  string `bun:"display_name"`
		IdentityType string `bun:"identity_type"`
	}, 0)
	if err := db.NewSelect().TableExpr("message_mentions AS mm").
		ColumnExpr("mm.message_id").
		ColumnExpr("oi.display_name").
		ColumnExpr("oi.type AS identity_type").
		Join("JOIN chat_subjects AS cs ON cs.id = mm.subject_id AND cs.organization_id = mm.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN organization_identities AS oi ON oi.id = cs.source_id AND oi.organization_id = cs.organization_id").
		Where("mm.organization_id = ?", organizationID).
		Where("mm.message_id IN (?)", bun.In(messageIDs)).
		OrderExpr("mm.message_id ASC, oi.display_name ASC").
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load group message mentions: %w", err)
	}
	for _, row := range rows {
		mentions[row.MessageID] = append(mentions[row.MessageID], groupMessageSender{Name: row.DisplayName, Kind: row.IdentityType})
	}
	return mentions, nil
}

// loadClaimedInputMessages 标记本次运行认领的输入各自来自哪条消息。
func loadClaimedInputMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) (map[string]bool, error) {
	sourceIDs := make([]string, 0)
	if err := db.NewSelect().Model((*servermodels.AgentInput)(nil)).
		ColumnExpr("ai.source_message_id").
		Where("ai.lane_id = ?", run.LaneID).
		Where("ai.input_seq BETWEEN ? AND ?", run.InputStartSeq, endSeq).
		Scan(ctx, &sourceIDs); err != nil {
		return nil, fmt.Errorf("load claimed input messages: %w", err)
	}
	addressed := make(map[string]bool, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		addressed[sourceID] = true
	}
	return addressed, nil
}

// applyMentions 从回复正文提取点名成员，保存提醒关系并为有执行资格的 AI 员工追加接力输入。
func (p groupMentionRunPolicy) applyMentions(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, run *servermodels.AgentRun, messageID, content string) error {
	participants, err := loadGroupMentionParticipants(ctx, db, run.OrganizationID, run.ConversationID, run.AgentIdentityID)
	if err != nil {
		return err
	}
	targets := make([]groupMentionTarget, 0)
	for _, name := range extractMentionNames(content, slices.Collect(maps.Keys(participants))) {
		// 重名成员无法确定点名对象，整体忽略。
		if len(participants[name]) != 1 {
			slog.Warn("群内点名的成员名称重复，已忽略",
				"organization_id", run.OrganizationID, "conversation_id", run.ConversationID,
				"agent_run_id", run.ID, "display_name", name)
			continue
		}
		targets = append(targets, participants[name][0])
	}
	if len(targets) == 0 {
		return nil
	}
	rows := make([]*servermodels.MessageMention, 0, len(targets))
	for _, target := range targets {
		rows = append(rows, &servermodels.MessageMention{
			OrganizationID: run.OrganizationID, MessageID: messageID, SubjectID: target.ChatSubjectID,
		})
	}
	if _, err := db.NewInsert().Model(&rows).
		Column("organization_id", "message_id", "subject_id").Exec(ctx); err != nil {
		return fmt.Errorf("create group agent message mentions: %w", err)
	}
	ordinal := 0
	for _, target := range targets {
		if target.IdentityType != string(domain.OrganizationIdentityTypeAgent) {
			continue
		}
		revisionID, eligible, err := loadGroupAgentRevision(ctx, db, run.OrganizationID, run.ConversationID, target.IdentityID, true)
		if err != nil {
			return err
		}
		if !eligible {
			slog.Warn("被接力点名的 AI 员工不满足执行资格",
				"organization_id", run.OrganizationID, "conversation_id", run.ConversationID,
				"agent_run_id", run.ID, "display_name", target.DisplayName)
			continue
		}
		if err := p.scheduler.appendInput(ctx, db, agentRunSpec{
			OrganizationID: run.OrganizationID, ConversationID: run.ConversationID,
			AgentIdentityID: target.IdentityID, RevisionID: revisionID,
			ScopeKind: domain.AgentExecutionScopeConversation, ScopeID: run.ConversationID,
			Kind: domain.AgentInputKindHandoff, SourceSubjectID: policyContext.AgentSubjectID, SourceOrdinal: ordinal,
		}, messageID); err != nil {
			return fmt.Errorf("append group handoff input: %w", err)
		}
		slog.Info("AI 员工点名接力已排队",
			"organization_id", run.OrganizationID, "conversation_id", run.ConversationID,
			"agent_run_id", run.ID, "target_agent_identity_id", target.IdentityID)
		ordinal++
	}
	return nil
}

type groupMentionTarget struct {
	ChatSubjectID string `bun:"chat_subject_id"`
	IdentityID    string `bun:"identity_id"`
	DisplayName   string `bun:"display_name"`
	IdentityType  string `bun:"identity_type"`
}

// loadGroupMentionParticipants 按显示名归集群内除自己以外的有效参与者。
func loadGroupMentionParticipants(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string) (map[string][]groupMentionTarget, error) {
	rows := make([]groupMentionTarget, 0)
	if err := db.NewSelect().TableExpr("conversation_participants AS cp").
		ColumnExpr("cs.id AS chat_subject_id").
		ColumnExpr("cs.source_id AS identity_id").
		ColumnExpr("oi.display_name").
		ColumnExpr("oi.type AS identity_type").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN organization_identities AS oi ON oi.id = cs.source_id AND oi.organization_id = cs.organization_id").
		Where("cp.organization_id = ? AND cp.conversation_id = ?", organizationID, conversationID).
		Where("cp.left_at IS NULL").
		Where("cs.source_id <> ?", agentIdentityID).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load group mention participants: %w", err)
	}
	participants := make(map[string][]groupMentionTarget, len(rows))
	for _, row := range rows {
		participants[row.DisplayName] = append(participants[row.DisplayName], row)
	}
	return participants, nil
}

// extractMentionNames 按出现顺序返回正文中去重后的点名成员，@ 须位于开头或空白之后且成员名后不紧跟字母、组合标记、数字或下划线。
func extractMentionNames(content string, names []string) []string {
	source := []byte(content)
	// 代码块、围栏信息和行内代码的内容替换为空格，其中的 @ 不形成点名。
	blank := func(segment text.Segment) {
		for i := segment.Start; i < segment.Stop; i++ {
			source[i] = ' '
		}
	}
	_ = ast.Walk(markdownParser.Parse(text.NewReader(source)), func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch code := node.(type) {
		case *ast.FencedCodeBlock:
			if code.Info != nil {
				blank(code.Info.Segment)
			}
			for i := 0; i < code.Lines().Len(); i++ {
				blank(code.Lines().At(i))
			}
		case *ast.CodeBlock:
			for i := 0; i < code.Lines().Len(); i++ {
				blank(code.Lines().At(i))
			}
		case *ast.CodeSpan:
			for child := code.FirstChild(); child != nil; child = child.NextSibling() {
				if span, ok := child.(*ast.Text); ok {
					blank(span.Segment)
				}
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	plain := string(source)
	// 较长的成员名优先匹配，前缀相同的成员按完整名称区分。
	slices.SortFunc(names, func(a, b string) int { return len(b) - len(a) })
	found := make([]string, 0)
	for index, char := range plain {
		if char != '@' {
			continue
		}
		if previous, _ := utf8.DecodeLastRuneInString(plain[:index]); index > 0 && !unicode.IsSpace(previous) {
			continue
		}
		rest := plain[index+1:]
		for _, name := range names {
			if !strings.HasPrefix(rest, name) {
				continue
			}
			if next, size := utf8.DecodeRuneInString(rest[len(name):]); size > 0 && (unicode.IsLetter(next) || unicode.IsMark(next) || unicode.IsNumber(next) || next == '_') {
				continue
			}
			if !slices.Contains(found, name) {
				found = append(found, name)
			}
			break
		}
	}
	return found
}
