//go:build server

// 会话业务操作依赖与共享错误映射。
package appservice

import (
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// conversationOps 持有会话与消息的 Action 和 Query。
type conversationOps struct {
	sendAttachmentMessage           *conversationaction.SendAttachmentMessageAction
	listConversationMessages        *conversationaction.ListConversationMessagesQuery
	updateConversationUnreadMark    *conversationaction.UpdateConversationUnreadMarkAction
	updateConversationPin           *conversationaction.UpdateConversationPinAction
	markConversationRead            *conversationaction.MarkConversationReadAction
	reportConversationTyping        *conversationaction.ReportConversationTypingAction
	conversationNavigation          *conversationaction.GetConversationNavigationStateQuery
	pendingConversationMentions     *conversationaction.ListPendingConversationMentionsQuery
	reviewConversationMention       *conversationaction.MarkConversationMentionReviewedAction
	updateConversationNotifications *conversationaction.UpdateConversationNotificationSettingsAction
	sendCustomerTextMessage         *conversationaction.SendCustomerTextMessageAction
	sendCustomerAttachmentMessage   *conversationaction.SendCustomerAttachmentMessageAction
	claimServiceSession             *conversationaction.ClaimServiceSessionAction
	transferServiceSession          *conversationaction.TransferServiceSessionAction
	closeServiceSession             *conversationaction.CloseServiceSessionAction
	reopenServiceSession            *conversationaction.ReopenServiceSessionAction
	sendFirstAgentTextMessage       *conversationaction.SendFirstAgentTextMessageAction
	sendAgentTextMessage            *conversationaction.SendAgentTextMessageAction
	listCustomerCopilotThreads      *conversationaction.ListCustomerCopilotThreadsQuery
	sendFirstCustomerCopilotMessage *conversationaction.SendFirstCustomerCopilotMessageAction
	sendCustomerCopilotTextMessage  *conversationaction.SendCustomerCopilotTextMessageAction
	sendFirstDirectTextMessage      *conversationaction.SendFirstDirectTextMessageAction
	findDirectConversation          *conversationaction.FindDirectConversationQuery
	sendDirectTextMessage           *conversationaction.SendDirectTextMessageAction
	createGroupConversation         *conversationaction.CreateGroupConversationAction
	getGroupConversation            *conversationaction.GetGroupConversationQuery
	updateGroupConversation         *conversationaction.UpdateGroupConversationAction
	addGroupConversationMembers     *conversationaction.AddGroupConversationMembersAction
	removeGroupConversationMember   *conversationaction.RemoveGroupConversationMemberAction
	transferGroupConversationOwner  *conversationaction.TransferGroupConversationOwnerAction
	leaveGroupConversation          *conversationaction.LeaveGroupConversationAction
	dissolveGroupConversation       *conversationaction.DissolveGroupConversationAction
	sendGroupTextMessage            *conversationaction.SendGroupTextMessageAction
	getAgentRunProcess              *conversationaction.GetAgentRunProcessQuery
	authorizeAgentRunStream         *conversationaction.AuthorizeAgentRunStreamQuery
}

// newConversationOps 创建会话与消息的业务实现依赖。
func newConversationOps(db *bun.DB, agentScheduler conversationaction.AgentMessageScheduler, agentCoordinator *agentrunaction.ExecuteAction, taskEnqueuer servertask.TxEnqueuer) conversationOps {
	return conversationOps{
		sendAttachmentMessage:           conversationaction.NewSendAttachmentMessageAction(db, agentScheduler),
		listConversationMessages:        conversationaction.NewListConversationMessagesQuery(db),
		updateConversationUnreadMark:    conversationaction.NewUpdateConversationUnreadMarkAction(db),
		updateConversationPin:           conversationaction.NewUpdateConversationPinAction(db),
		markConversationRead:            conversationaction.NewMarkConversationReadAction(db),
		reportConversationTyping:        conversationaction.NewReportConversationTypingAction(db),
		conversationNavigation:          conversationaction.NewGetConversationNavigationStateQuery(db),
		pendingConversationMentions:     conversationaction.NewListPendingConversationMentionsQuery(db),
		reviewConversationMention:       conversationaction.NewMarkConversationMentionReviewedAction(db),
		updateConversationNotifications: conversationaction.NewUpdateConversationNotificationSettingsAction(db),
		sendCustomerTextMessage:         conversationaction.NewSendCustomerTextMessageAction(db, taskEnqueuer),
		sendCustomerAttachmentMessage:   conversationaction.NewSendCustomerAttachmentMessageAction(db, taskEnqueuer),
		claimServiceSession:             conversationaction.NewClaimServiceSessionAction(db, agentCoordinator, taskEnqueuer),
		transferServiceSession:          conversationaction.NewTransferServiceSessionAction(db, agentCoordinator, agentScheduler, taskEnqueuer),
		closeServiceSession:             conversationaction.NewCloseServiceSessionAction(db, agentCoordinator, taskEnqueuer),
		reopenServiceSession:            conversationaction.NewReopenServiceSessionAction(db),
		sendFirstAgentTextMessage:       conversationaction.NewSendFirstAgentTextMessageAction(db, agentScheduler),
		sendAgentTextMessage:            conversationaction.NewSendAgentTextMessageAction(db, agentScheduler),
		listCustomerCopilotThreads:      conversationaction.NewListCustomerCopilotThreadsQuery(db),
		sendFirstCustomerCopilotMessage: conversationaction.NewSendFirstCustomerCopilotMessageAction(db, agentScheduler),
		sendCustomerCopilotTextMessage:  conversationaction.NewSendCustomerCopilotTextMessageAction(db, agentScheduler),
		sendFirstDirectTextMessage:      conversationaction.NewSendFirstDirectTextMessageAction(db),
		findDirectConversation:          conversationaction.NewFindDirectConversationQuery(db),
		sendDirectTextMessage:           conversationaction.NewSendDirectTextMessageAction(db),
		createGroupConversation:         conversationaction.NewCreateGroupConversationAction(db),
		getGroupConversation:            conversationaction.NewGetGroupConversationQuery(db),
		updateGroupConversation:         conversationaction.NewUpdateGroupConversationAction(db),
		addGroupConversationMembers:     conversationaction.NewAddGroupConversationMembersAction(db),
		removeGroupConversationMember:   conversationaction.NewRemoveGroupConversationMemberAction(db, agentCoordinator),
		transferGroupConversationOwner:  conversationaction.NewTransferGroupConversationOwnerAction(db),
		leaveGroupConversation:          conversationaction.NewLeaveGroupConversationAction(db, agentCoordinator),
		dissolveGroupConversation:       conversationaction.NewDissolveGroupConversationAction(db, agentCoordinator),
		sendGroupTextMessage:            conversationaction.NewSendGroupTextMessageAction(db, agentScheduler),
		getAgentRunProcess:              conversationaction.NewGetAgentRunProcessQuery(db),
		authorizeAgentRunStream:         conversationaction.NewAuthorizeAgentRunStreamQuery(db),
	}
}

// assistantConflictKeys 是助理无法接收新请求时的冲突提示。
var assistantConflictKeys = map[string]cervii18n.Key{
	conversationaction.ConflictReasonAssistantPaused:   cervii18n.ErrorAssistantPaused,
	conversationaction.ConflictReasonAssistantUnbound:  cervii18n.ErrorAssistantUnbound,
	conversationaction.ConflictReasonAssistantInactive: cervii18n.ErrorAssistantInactive,
}

var conversationMessageValidationKeys = map[conversationaction.ValidationCode]cervii18n.Key{
	conversationaction.ValidationConversationIDInvalid:     cervii18n.FieldConversationIDInvalid,
	conversationaction.ValidationClientMessageIDInvalid:    cervii18n.FieldClientMessageIDInvalid,
	conversationaction.ValidationLastReadMessageIDInvalid:  cervii18n.FieldClientMessageIDInvalid,
	conversationaction.ValidationReplyToMessageIDInvalid:   cervii18n.FieldReplyToMessageIDInvalid,
	conversationaction.ValidationMentionSubjectIDsInvalid:  cervii18n.FieldMentionSubjectIDsInvalid,
	conversationaction.ValidationMentionIdentityIDsInvalid: cervii18n.FieldMentionIdentityIDsInvalid,
	conversationaction.ValidationBodyRequired:              cervii18n.FieldMessageBodyRequired,
	conversationaction.ValidationBodyTooLong:               cervii18n.FieldMessageBodyTooLong,
	conversationaction.ValidationCursorInvalid:             cervii18n.FieldMessageCursorInvalid,
	conversationaction.ValidationMessageVisibilityInvalid:  cervii18n.FieldMessageVisibilityInvalid,
	conversationaction.ValidationFileIDInvalid:             cervii18n.ErrorFileNotFound,
	conversationaction.ValidationTargetIdentityIDInvalid:   cervii18n.FieldTargetIdentityIDInvalid,
	conversationaction.ValidationTargetTeamIDInvalid:       cervii18n.FieldTargetTeamIDInvalid,
	conversationaction.ValidationTransferTargetKindInvalid: cervii18n.FieldTransferTargetInvalid,
	conversationaction.ValidationGroupTitleTooLong:         cervii18n.FieldGroupTitleTooLong,
	conversationaction.ValidationGroupDescriptionTooLong:   cervii18n.FieldGroupDescriptionTooLong,
	conversationaction.ValidationGroupImageFileIDInvalid:   cervii18n.FieldGroupImageFileIDInvalid,
	conversationaction.ValidationGroupMembersRequired:      cervii18n.FieldGroupMembersRequired,
	conversationaction.ValidationGroupMembersTooMany:       cervii18n.FieldGroupMembersTooMany,
	conversationaction.ValidationGroupMemberIDsInvalid:     cervii18n.FieldGroupMemberIDsInvalid,
	conversationaction.ValidationGroupMemberIDInvalid:      cervii18n.FieldGroupMemberIDInvalid,
	conversationaction.ValidationGroupOwnerIDInvalid:       cervii18n.FieldGroupOwnerIDInvalid,
	conversationaction.ValidationNeighborIDInvalid:         cervii18n.FieldConversationPinTargetInvalid,
	conversationaction.ValidationPinPositionInvalid:        cervii18n.FieldConversationPinTargetInvalid,
	conversationaction.ValidationPinOrderVersionInvalid:    cervii18n.FieldConversationPinTargetInvalid,
}
