//go:build server

// 会话业务操作依赖与共享错误映射。
package appservice

import (
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	directchataction "github.com/runforyou-ai/cervi/internal/actions/directchat"
	groupchataction "github.com/runforyou-ai/cervi/internal/actions/groupchat"
	servicesessionaction "github.com/runforyou-ai/cervi/internal/actions/servicesession"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// conversationOps 持有会话与消息的 Action 和 Query。
type conversationOps struct {
	sendAttachmentMessage           *directchataction.SendAttachmentMessageAction
	listConversationMessages        *conversationaction.ListConversationMessagesQuery
	updateConversationUnreadMark    *conversationaction.UpdateConversationUnreadMarkAction
	updateConversationPin           *conversationaction.UpdateConversationPinAction
	markConversationRead            *conversationaction.MarkConversationReadAction
	reportConversationTyping        *conversationaction.ReportConversationTypingAction
	conversationNavigation          *conversationaction.GetConversationNavigationStateQuery
	pendingConversationMentions     *conversationaction.ListPendingConversationMentionsQuery
	reviewConversationMention       *conversationaction.MarkConversationMentionReviewedAction
	updateConversationNotifications *conversationaction.UpdateConversationNotificationSettingsAction
	sendServiceTextMessage          *servicesessionaction.SendServiceTextMessageAction
	sendServiceAttachmentMessage    *servicesessionaction.SendServiceAttachmentMessageAction
	claimServiceSession             *servicesessionaction.ClaimServiceSessionAction
	transferServiceSession          *servicesessionaction.TransferServiceSessionAction
	closeServiceSession             *servicesessionaction.CloseServiceSessionAction
	reopenServiceSession            *servicesessionaction.ReopenServiceSessionAction
	sendFirstAgentTextMessage       *directchataction.SendFirstAgentTextMessageAction
	sendAgentTextMessage            *directchataction.SendAgentTextMessageAction
	listServiceCopilotThreads       *directchataction.ListServiceCopilotThreadsQuery
	sendFirstServiceCopilotMessage  *directchataction.SendFirstServiceCopilotMessageAction
	sendServiceCopilotTextMessage   *directchataction.SendServiceCopilotTextMessageAction
	sendFirstDirectTextMessage      *directchataction.SendFirstDirectTextMessageAction
	findDirectConversation          *directchataction.FindDirectConversationQuery
	sendDirectTextMessage           *directchataction.SendDirectTextMessageAction
	createGroupConversation         *groupchataction.CreateGroupConversationAction
	getGroupConversation            *groupchataction.GetGroupConversationQuery
	updateGroupConversation         *groupchataction.UpdateGroupConversationAction
	addGroupConversationMembers     *groupchataction.AddGroupConversationMembersAction
	removeGroupConversationMember   *groupchataction.RemoveGroupConversationMemberAction
	transferGroupConversationOwner  *groupchataction.TransferGroupConversationOwnerAction
	leaveGroupConversation          *groupchataction.LeaveGroupConversationAction
	dissolveGroupConversation       *groupchataction.DissolveGroupConversationAction
	sendGroupTextMessage            *groupchataction.SendGroupTextMessageAction
	getAgentRunProcess              *conversationaction.GetAgentRunProcessQuery
	authorizeAgentRunStream         *conversationaction.AuthorizeAgentRunStreamQuery
}

// newConversationOps 创建会话与消息的业务实现依赖。
func newConversationOps(db *bun.DB, agentScheduler conversationaction.AgentMessageScheduler, agentCoordinator *agentrunaction.ExecuteAction, taskEnqueuer servertask.TxEnqueuer) conversationOps {
	return conversationOps{
		sendAttachmentMessage:           directchataction.NewSendAttachmentMessageAction(db, agentScheduler),
		listConversationMessages:        conversationaction.NewListConversationMessagesQuery(db),
		updateConversationUnreadMark:    conversationaction.NewUpdateConversationUnreadMarkAction(db),
		updateConversationPin:           conversationaction.NewUpdateConversationPinAction(db),
		markConversationRead:            conversationaction.NewMarkConversationReadAction(db),
		reportConversationTyping:        conversationaction.NewReportConversationTypingAction(db),
		conversationNavigation:          conversationaction.NewGetConversationNavigationStateQuery(db),
		pendingConversationMentions:     conversationaction.NewListPendingConversationMentionsQuery(db),
		reviewConversationMention:       conversationaction.NewMarkConversationMentionReviewedAction(db),
		updateConversationNotifications: conversationaction.NewUpdateConversationNotificationSettingsAction(db),
		sendServiceTextMessage:          servicesessionaction.NewSendServiceTextMessageAction(db, taskEnqueuer),
		sendServiceAttachmentMessage:    servicesessionaction.NewSendServiceAttachmentMessageAction(db, taskEnqueuer),
		claimServiceSession:             servicesessionaction.NewClaimServiceSessionAction(db, agentCoordinator, taskEnqueuer),
		transferServiceSession:          servicesessionaction.NewTransferServiceSessionAction(db, agentCoordinator, agentScheduler, taskEnqueuer),
		closeServiceSession:             servicesessionaction.NewCloseServiceSessionAction(db, agentCoordinator, taskEnqueuer),
		reopenServiceSession:            servicesessionaction.NewReopenServiceSessionAction(db),
		sendFirstAgentTextMessage:       directchataction.NewSendFirstAgentTextMessageAction(db, agentScheduler),
		sendAgentTextMessage:            directchataction.NewSendAgentTextMessageAction(db, agentScheduler),
		listServiceCopilotThreads:       directchataction.NewListServiceCopilotThreadsQuery(db),
		sendFirstServiceCopilotMessage:  directchataction.NewSendFirstServiceCopilotMessageAction(db, agentScheduler),
		sendServiceCopilotTextMessage:   directchataction.NewSendServiceCopilotTextMessageAction(db, agentScheduler),
		sendFirstDirectTextMessage:      directchataction.NewSendFirstDirectTextMessageAction(db),
		findDirectConversation:          directchataction.NewFindDirectConversationQuery(db),
		sendDirectTextMessage:           directchataction.NewSendDirectTextMessageAction(db),
		createGroupConversation:         groupchataction.NewCreateGroupConversationAction(db),
		getGroupConversation:            groupchataction.NewGetGroupConversationQuery(db),
		updateGroupConversation:         groupchataction.NewUpdateGroupConversationAction(db),
		addGroupConversationMembers:     groupchataction.NewAddGroupConversationMembersAction(db),
		removeGroupConversationMember:   groupchataction.NewRemoveGroupConversationMemberAction(db, agentCoordinator),
		transferGroupConversationOwner:  groupchataction.NewTransferGroupConversationOwnerAction(db),
		leaveGroupConversation:          groupchataction.NewLeaveGroupConversationAction(db, agentCoordinator),
		dissolveGroupConversation:       groupchataction.NewDissolveGroupConversationAction(db, agentCoordinator),
		sendGroupTextMessage:            groupchataction.NewSendGroupTextMessageAction(db, agentScheduler),
		getAgentRunProcess:              conversationaction.NewGetAgentRunProcessQuery(db),
		authorizeAgentRunStream:         conversationaction.NewAuthorizeAgentRunStreamQuery(db),
	}
}

// assistantConflictKeys 是助理无法接收新请求时的冲突提示。
var assistantConflictKeys = map[string]cervii18n.Key{
	conversationaction.ConflictReasonAssistantPaused:  cervii18n.ErrorAssistantPaused,
	conversationaction.ConflictReasonAssistantUnbound: cervii18n.ErrorAssistantUnbound,
	groupchataction.ConflictReasonAssistantInactive:   cervii18n.ErrorAssistantInactive,
}

var conversationMessageValidationKeys = map[conversationaction.ValidationCode]cervii18n.Key{
	conversationaction.ValidationConversationIDInvalid:       cervii18n.FieldConversationIDInvalid,
	conversationaction.ValidationClientMessageIDInvalid:      cervii18n.FieldClientMessageIDInvalid,
	conversationaction.ValidationLastReadMessageIDInvalid:    cervii18n.FieldClientMessageIDInvalid,
	conversationaction.ValidationReplyToMessageIDInvalid:     cervii18n.FieldReplyToMessageIDInvalid,
	groupchataction.ValidationMentionSubjectIDsInvalid:       cervii18n.FieldMentionSubjectIDsInvalid,
	servicesessionaction.ValidationMentionIdentityIDsInvalid: cervii18n.FieldMentionIdentityIDsInvalid,
	conversationaction.ValidationBodyRequired:                cervii18n.FieldMessageBodyRequired,
	conversationaction.ValidationBodyTooLong:                 cervii18n.FieldMessageBodyTooLong,
	servicesessionaction.ValidationTranslationInvalid:        cervii18n.FieldMessageTranslationInvalid,
	conversationaction.ValidationCursorInvalid:               cervii18n.FieldMessageCursorInvalid,
	servicesessionaction.ValidationMessageVisibilityInvalid:  cervii18n.FieldMessageVisibilityInvalid,
	conversationaction.ValidationFileIDInvalid:               cervii18n.ErrorFileNotFound,
	conversationaction.ValidationTargetIdentityIDInvalid:     cervii18n.FieldTargetIdentityIDInvalid,
	servicesessionaction.ValidationTargetTeamIDInvalid:       cervii18n.FieldTargetTeamIDInvalid,
	servicesessionaction.ValidationTransferTargetKindInvalid: cervii18n.FieldTransferTargetInvalid,
	groupchataction.ValidationGroupTitleTooLong:              cervii18n.FieldGroupTitleTooLong,
	groupchataction.ValidationGroupDescriptionTooLong:        cervii18n.FieldGroupDescriptionTooLong,
	groupchataction.ValidationGroupImageFileIDInvalid:        cervii18n.FieldGroupImageFileIDInvalid,
	groupchataction.ValidationGroupMembersRequired:           cervii18n.FieldGroupMembersRequired,
	groupchataction.ValidationGroupMembersTooMany:            cervii18n.FieldGroupMembersTooMany,
	groupchataction.ValidationGroupMemberIDsInvalid:          cervii18n.FieldGroupMemberIDsInvalid,
	groupchataction.ValidationGroupMemberIDInvalid:           cervii18n.FieldGroupMemberIDInvalid,
	groupchataction.ValidationGroupOwnerIDInvalid:            cervii18n.FieldGroupOwnerIDInvalid,
	conversationaction.ValidationNeighborIDInvalid:           cervii18n.FieldConversationPinTargetInvalid,
	conversationaction.ValidationPinPositionInvalid:          cervii18n.FieldConversationPinTargetInvalid,
	conversationaction.ValidationPinOrderVersionInvalid:      cervii18n.FieldConversationPinTargetInvalid,
}
