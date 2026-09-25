//go:build server

// 消息读取、游标解析与消息响应转换。
package appservice

import (
	"context"
	"errors"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"log/slog"
	"strconv"
	"strings"
)

// ListConversationMessages 返回成员可见的会话消息。
func (o *directOperations) ListConversationMessages(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input ConversationMessageListInput) (ConversationMessageList, error) {
	actionInput := conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID}
	if input.Before != "" && input.After != "" {
		return ConversationMessageList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"cursor": cervii18n.FieldMessageCursorInvalid})
	}
	if input.Before != "" {
		point, valid := decodeConversationMessageCursor(input.Before, conversationID)
		if !valid {
			return ConversationMessageList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"before": cervii18n.FieldMessageCursorInvalid})
		}
		actionInput.Before = &point
	}
	if input.After != "" {
		point, valid := decodeConversationMessageCursor(input.After, conversationID)
		if !valid {
			return ConversationMessageList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"after": cervii18n.FieldMessageCursorInvalid})
		}
		actionInput.After = &point
	}

	history, err := o.listConversationMessages.Execute(ctx, identity, actionInput)
	if err != nil {
		return ConversationMessageList{}, conversationMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return o.conversationMessageListFromAction(ctx, meta, identity, conversationID, history)
}

// ReadConversationMessageWindow 重读已加载首尾游标之间的完整消息范围。
func (o *directOperations) ReadConversationMessageWindow(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input ConversationMessageWindowInput) (ConversationMessageList, error) {
	start, validStart := decodeConversationMessageCursor(input.Start, conversationID)
	end, validEnd := decodeConversationMessageCursor(input.End, conversationID)
	if !validStart || !validEnd || start.MessageSeq > end.MessageSeq {
		return ConversationMessageList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"cursor": cervii18n.FieldMessageCursorInvalid})
	}
	history, err := o.listConversationMessages.Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID, Start: &start, End: &end})
	if err != nil {
		return ConversationMessageList{}, conversationMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return o.conversationMessageListFromAction(ctx, meta, identity, conversationID, history)
}

// conversationMessageFromAction 转换成员会话消息契约。
func conversationMessageFromAction(message conversationaction.ConversationMessage, avatarURLs map[string]string) ConversationMessage {
	sender := conversationMessageSenderFromAction(message.Sender, avatarURLs)
	var sessionStart *ConversationMessageSessionStart
	if message.SessionStart != nil {
		sessionStart = &ConversationMessageSessionStart{
			Sequence:  message.SessionStart.Sequence,
			StartedAt: message.SessionStart.StartedAt,
			Status:    ServiceSessionStatus(message.SessionStart.Status),
		}
	}
	var systemEvent *ConversationSystemEvent
	if message.SystemEvent != nil {
		targets := make([]ConversationSystemEventParticipant, 0, len(message.SystemEvent.Targets))
		for _, target := range message.SystemEvent.Targets {
			targets = append(targets, ConversationSystemEventParticipant{IdentityID: target.IdentityID, DisplayName: target.DisplayName})
		}
		systemEvent = &ConversationSystemEvent{
			Type: ConversationSystemEventType(message.SystemEvent.Type),
			Actor: ConversationSystemEventParticipant{
				IdentityID: message.SystemEvent.Actor.IdentityID, DisplayName: message.SystemEvent.Actor.DisplayName,
			},
			Targets: targets, PreviousTitle: message.SystemEvent.PreviousTitle, Title: message.SystemEvent.Title,
			ServiceSessionID: message.SystemEvent.ServiceSessionID, FromIdentityID: message.SystemEvent.FromIdentityID,
			FromDisplayName: message.SystemEvent.FromDisplayName, HandoffReason: (*AgentHandoffReason)(message.SystemEvent.Reason),
			ReturnReason: (*ServiceSessionReturnReason)(message.SystemEvent.ReturnReason),
			CloseReason:  (*ServiceSessionCloseReason)(message.SystemEvent.CloseReason),
			ReasonText:   message.SystemEvent.ReasonText, CategoryName: message.SystemEvent.CategoryName, AgentRunID: message.SystemEvent.AgentRunID,
			RatingResolved: message.SystemEvent.Resolved, RatingComment: message.SystemEvent.Comment, Email: message.SystemEvent.Email,
		}
		// 客服处理周期事件的操作人按群聊事件的 actor 结构返回。
		if message.SystemEvent.ActorIdentityID != nil && message.SystemEvent.ActorDisplayName != nil {
			systemEvent.Actor = ConversationSystemEventParticipant{IdentityID: *message.SystemEvent.ActorIdentityID, DisplayName: *message.SystemEvent.ActorDisplayName}
		}
		if target := message.SystemEvent.Target; target != nil {
			systemEvent.SessionTarget = &ServiceSessionTarget{Kind: ServiceSessionTargetKind(target.Kind),
				TeamID: target.TeamID, TeamName: target.TeamName, IdentityID: target.IdentityID, DisplayName: target.DisplayName}
		}
	}
	var replyTo *ConversationMessageReference
	if message.ReplyTo != nil {
		replyTo = &ConversationMessageReference{
			ExternalSenderName: message.ReplyTo.ExternalSenderName, ID: message.ReplyTo.ID, Type: MessageType(message.ReplyTo.Type), Visibility: MessageVisibility(message.ReplyTo.Visibility), Body: message.ReplyTo.Body, Deleted: message.ReplyTo.Deleted,
			Sender: conversationMessageSenderFromAction(message.ReplyTo.Sender, avatarURLs),
		}
	}
	mentions := make([]ConversationMessageMention, 0, len(message.Mentions))
	for _, mention := range message.Mentions {
		mentions = append(mentions, ConversationMessageMention{
			ChatSubjectID: mention.ChatSubjectID, Kind: ChatSubjectKind(mention.Kind),
			SourceID: mention.SourceID, DisplayName: mention.DisplayName,
		})
	}
	var attachment *MessageAttachment
	if message.Attachment != nil {
		attachment = &MessageAttachment{
			File:           File{ID: message.Attachment.ID, Name: message.Attachment.Name, ContentType: message.Attachment.ContentType, ByteSize: message.Attachment.ByteSize},
			TransferStatus: MessageAttachmentTransferStatus(message.Attachment.TransferStatus),
			ImageWidth:     message.Attachment.ImageWidth, ImageHeight: message.Attachment.ImageHeight,
		}
	}
	var translation *ConversationMessageTranslation
	if message.Translation != nil {
		translation = &ConversationMessageTranslation{Language: message.Translation.Language, Body: message.Translation.Body}
	}
	// 文本和附件消息可以被引用；对客回复只能引用对客可见且渠道能够投递该引用的消息。
	quotable := message.Type == domain.MessageTypeText || message.Type == domain.MessageTypeAttachment
	return ConversationMessage{
		CanReply:        quotable && !message.ReplyUnavailable && message.Visibility != domain.MessageVisibilityInternalOnly,
		CanNoteReply:    quotable,
		ClientMessageID: message.ClientMessageID,
		Attachment:      attachment,
		AgentProcess:    conversationAgentProcessFromAction(message.AgentProcess),
		ID:              message.ID, Type: MessageType(message.Type), Visibility: MessageVisibility(message.Visibility), Body: message.Body,
		Language: common.StringValue(message.Language), Translation: translation,
		OriginatedAt: message.OriginatedAt, SourceOrder: message.SourceOrder, CreatedAt: message.CreatedAt, MessageSeq: strconv.FormatInt(message.MessageSeq, 10),
		Sender: sender, SessionStart: sessionStart, SystemEvent: systemEvent,
		ReplyTo: replyTo, Mentions: mentions, MentionAll: message.MentionAll,
	}
}

// conversationMessageSenderFromAction 转换消息发送主体。
func conversationMessageSenderFromAction(sender *conversationaction.ConversationMessageSender, avatarURLs map[string]string) *ConversationMessageSender {
	if sender == nil {
		return nil
	}
	return &ConversationMessageSender{
		ChatSubjectID: sender.ChatSubjectID, Kind: ChatSubjectKind(sender.Kind),
		SourceID: sender.SourceID, DisplayName: sender.DisplayName,
		AvatarURL: optionalFileURL(avatarURLs, sender.AvatarFileID), IdentityType: (*OrganizationIdentityType)(sender.IdentityType),
	}
}

// encodeConversationMessageCursor 编码绑定会话的消息序号与定位编号。
func encodeConversationMessageCursor(conversationID string, point conversationaction.MessageCursorPoint) string {
	return conversationID + "." + strconv.FormatInt(point.MessageSeq, 10) + "." + point.ID
}

// decodeConversationMessageCursor 校验消息游标的会话、序号和定位编号。
func decodeConversationMessageCursor(value, conversationID string) (conversationaction.MessageCursorPoint, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != conversationID || !common.ValidUUID(parts[2]) {
		return conversationaction.MessageCursorPoint{}, false
	}
	sequence, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || sequence <= 0 {
		return conversationaction.MessageCursorPoint{}, false
	}
	return conversationaction.MessageCursorPoint{ID: parts[2], MessageSeq: sequence}, true
}

// conversationMessageError 转换成员消息读取错误。
func conversationMessageError(ctx context.Context, meta RequestMeta, err error, organizationID, conversationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound).WithReason("conversation_unavailable")
	}
	if errors.Is(err, conversationaction.ErrMessageUnavailable) {
		return NotFoundError(meta, cervii18n.ErrorConversationMessageUnavailable).WithReason("message_unavailable")
	}
	if errors.Is(err, conversationaction.ErrMentionTargetInvalid) {
		return InvalidError(meta, cervii18n.ErrorConversationMentionTargetInvalid, nil)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	slog.Warn("读取会话消息失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorConversationMessageListFailed)
}

// conversationMessageListFromAction 共用成员消息窗口及游标转换。
func (o *directOperations) conversationMessageListFromAction(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, history conversationaction.ConversationMessageHistory) (ConversationMessageList, error) {
	agentAvatarFileIDs := make([]*string, 0, len(history.PendingAgents)+len(history.AgentRuns))
	for _, run := range history.AgentRuns {
		agentAvatarFileIDs = append(agentAvatarFileIDs, run.AgentAvatarFileID)
	}
	for _, agent := range history.PendingAgents {
		agentAvatarFileIDs = append(agentAvatarFileIDs, agent.AvatarFileID)
	}
	avatarURLs, err := o.conversationAvatarURLs(ctx, identity, history.Messages, agentAvatarFileIDs...)
	if err != nil {
		return ConversationMessageList{}, conversationMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	result := ConversationMessageList{HasEarlier: history.HasEarlier, HasLater: history.HasLater, Messages: make([]ConversationMessage, 0, len(history.Messages))}
	result.AgentRuns = make([]ConversationAgentRun, 0, len(history.AgentRuns))
	for _, run := range history.AgentRuns {
		result.AgentRuns = append(result.AgentRuns, ConversationAgentRun{ID: run.ID, AgentIdentityID: run.AgentIdentityID,
			AgentName: run.AgentName, AgentAvatarURL: optionalFileURL(avatarURLs, run.AgentAvatarFileID),
			Status: AgentRunStatus(run.Status), ErrorCode: run.ErrorCode, LastError: run.LastError,
			Process: conversationAgentProcessFromAction(run.Process), ExecutionDeviceName: run.ExecutionDeviceName})
	}
	result.PendingAgents = make([]ConversationPendingAgent, 0, len(history.PendingAgents))
	for _, agent := range history.PendingAgents {
		result.PendingAgents = append(result.PendingAgents, ConversationPendingAgent{
			IdentityID: agent.IdentityID, DisplayName: agent.DisplayName, AvatarURL: optionalFileURL(avatarURLs, agent.AvatarFileID),
		})
	}
	for _, message := range history.Messages {
		result.Messages = append(result.Messages, conversationMessageFromAction(message, avatarURLs))
	}
	if history.Before != nil {
		value := encodeConversationMessageCursor(conversationID, *history.Before)
		result.Before = &value
	}
	if history.After != nil {
		value := encodeConversationMessageCursor(conversationID, *history.After)
		result.After = &value
	}
	return result, nil
}

// conversationAvatarURLs 批量解析消息发送者、引用发送者、单聊目标和运行中 Agent 的头像。
func (o *directOperations) conversationAvatarURLs(ctx context.Context, identity *servermodels.Identity, messages []conversationaction.ConversationMessage, extraFileIDs ...*string) (map[string]string, error) {
	fileIDs := make([]string, 0, len(messages)+len(extraFileIDs))
	for _, fileID := range extraFileIDs {
		if fileID != nil {
			fileIDs = append(fileIDs, *fileID)
		}
	}
	for _, message := range messages {
		if message.Sender != nil && message.Sender.AvatarFileID != nil {
			fileIDs = append(fileIDs, *message.Sender.AvatarFileID)
		}
		if message.ReplyTo != nil && message.ReplyTo.Sender != nil && message.ReplyTo.Sender.AvatarFileID != nil {
			fileIDs = append(fileIDs, *message.ReplyTo.Sender.AvatarFileID)
		}
	}
	return o.activeFileURLs(ctx, identity, fileIDs)
}

// conversationMessageWithAvatar 转换发送结果并补充头像地址。
func (o *directOperations) conversationMessageWithAvatar(ctx context.Context, identity *servermodels.Identity, message conversationaction.ConversationMessage) ConversationMessage {
	urls, err := o.conversationAvatarURLs(ctx, identity, []conversationaction.ConversationMessage{message})
	if err != nil {
		slog.Warn("读取已保存消息头像失败", "organization_id", identity.Organization.ID, "message_id", message.ID, "error", err)
	}
	return conversationMessageFromAction(message, urls)
}
