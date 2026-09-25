//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ListServiceCopilotThreads 返回服务会话的全部 Copilot 线程。
func (o *directOperations) ListServiceCopilotThreads(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string) (ServiceCopilotThreadList, error) {
	threads, err := o.listServiceCopilotThreads.Execute(ctx, identity, conversationID)
	if err != nil {
		if ctx.Err() != nil {
			return ServiceCopilotThreadList{}, ctx.Err()
		}
		if errors.Is(err, conversationaction.ErrConversationNotFound) {
			return ServiceCopilotThreadList{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound)
		}
		slog.Warn("读取 Copilot 线程失败", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "error", err)
		return ServiceCopilotThreadList{}, FailedError(meta, cervii18n.ErrorServiceCopilotThreadListFailed)
	}
	fileIDs := make([]*string, 0, len(threads))
	for _, thread := range threads {
		fileIDs = append(fileIDs, thread.AgentAvatarFileID)
	}
	urls, err := o.conversationAvatarURLs(ctx, identity, nil, fileIDs...)
	if err != nil {
		slog.Warn("读取 Copilot 线程 AI 员工头像失败", "conversation_id", conversationID, "error", err)
	}
	output := make([]ServiceCopilotThread, 0, len(threads))
	for _, thread := range threads {
		output = append(output, serviceCopilotThreadFromAction(thread, urls))
	}
	return ServiceCopilotThreadList{Threads: output}, nil
}

// SendFirstServiceCopilotMessage 保存首条提问并确认新建的 Copilot 线程。
func (o *directOperations) SendFirstServiceCopilotMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input FirstServiceCopilotMessageInput) (FirstServiceCopilotMessageResult, error) {
	result, err := o.sendFirstServiceCopilotMessage.Execute(ctx, identity, conversationaction.FirstServiceCopilotMessageInput{
		ThreadID: input.ThreadID, ServedConversationID: conversationID, AgentIdentityID: input.AgentIdentityID,
		ClientMessageID: input.ClientMessageID, Body: input.Body,
	})
	if err != nil {
		return FirstServiceCopilotMessageResult{}, individualConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "start_copilot")
	}
	slog.Info("Copilot 线程首条提问已保存", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "thread_id", result.Thread.ID, "message_id", result.Message.ID, "agent_identity_id", result.Thread.AgentIdentityID)
	urls, err := o.conversationAvatarURLs(ctx, identity, []conversationaction.ConversationMessage{result.Message}, result.Thread.AgentAvatarFileID)
	if err != nil {
		slog.Warn("读取 Copilot 线程头像失败", "thread_id", result.Thread.ID, "error", err)
	}
	return FirstServiceCopilotMessageResult{
		Thread:  serviceCopilotThreadFromAction(result.Thread, urls),
		Message: conversationMessageFromAction(result.Message, urls),
	}, nil
}

// SendServiceCopilotTextMessage 保存当前成员在 Copilot 线程中的提问。
func (o *directOperations) SendServiceCopilotTextMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, threadID string, input ServiceCopilotTextMessageInput) (ConversationMessage, error) {
	message, err := o.sendServiceCopilotTextMessage.Execute(ctx, identity, conversationaction.InternalTextMessageInput{ConversationID: threadID, ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID})
	if err != nil {
		return ConversationMessage{}, individualConversationError(ctx, meta, err, identity.Organization.ID, threadID, "send_copilot")
	}
	slog.Info("Copilot 线程提问已保存", "organization_id", identity.Organization.ID, "thread_id", threadID, "message_id", message.ID)
	return o.conversationMessageWithAvatar(ctx, identity, message), nil
}

// StopServiceCopilotReply 停止 Copilot 线程中指定的回复。
func (o *directOperations) StopServiceCopilotReply(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, threadID, runID string) (AgentRunStatus, error) {
	status, err := o.agentCoordinator.StopServiceCopilotReply(ctx, identity, threadID, runID)
	if err == nil {
		return AgentRunStatus(status), nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return "", SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return "", NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	slog.Warn("停止 Copilot 回复失败", "organization_id", identity.Organization.ID, "thread_id", threadID, "agent_run_id", runID, "error", err)
	return "", FailedError(meta, cervii18n.ErrorAgentReplyStopFailed)
}

// serviceCopilotThreadFromAction 转换线程摘要并补充 AI 员工头像地址。
func serviceCopilotThreadFromAction(thread conversationaction.ServiceCopilotThread, avatarURLs map[string]string) ServiceCopilotThread {
	output := ServiceCopilotThread{
		ID: thread.ID, Title: thread.Title, AgentIdentityID: thread.AgentIdentityID, AgentName: thread.AgentName,
		AgentActive: thread.AgentStatus == domain.UserStatusActive, CreatedByIdentityID: thread.CreatedByIdentityID,
		CreatedByName: thread.CreatedByName, CreatedAt: thread.CreatedAt, LastActivityAt: thread.LastActivityAt,
	}
	if thread.AgentAvatarFileID != nil {
		output.AgentAvatarURL = avatarURLs[*thread.AgentAvatarFileID]
	}
	return output
}
