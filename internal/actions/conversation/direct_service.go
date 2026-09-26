//go:build server

package conversation

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// directServiceSession 在调用方持有 AI 聊天会话锁的事务中为发起人消息取得服务周期：进行中的周期继续承接；没有进行中的周期且 AI 员工服务员工时开启由该 AI 员工负责的新周期，首次开启时建立来源为 Cervi 单聊的服务会话；其余情况返回空，消息按普通 AI 聊天处理。
func directServiceSession(ctx context.Context, db bun.IDB, organizationID string, sendContext internalMessageContext, openingMessageID string, openedAt time.Time) (*servermodels.ServiceSession, error) {
	conversationID := sendContext.Conversation.ID
	service, err := chatstate.LoadServiceConversation(ctx, db, organizationID, conversationID)
	if errors.Is(err, chatstate.ErrConversationNotFound) {
		service = nil
	} else if err != nil {
		return nil, err
	}
	if service != nil {
		session, err := chatstate.LockOpenServiceSession(ctx, db, organizationID, conversationID)
		if err != nil || session != nil {
			return session, err
		}
	}
	serves, err := identityaction.ApplyDirectServiceAgentConditions(db.NewSelect().
		TableExpr("organization_identities AS oi").ColumnExpr("1").
		Where("oi.organization_id = ? AND oi.id = ?", organizationID, sendContext.AgentIdentityID), conversationID).
		Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("check direct service agent: %w", err)
	}
	if !serves {
		return nil, nil
	}
	if service == nil {
		if err := chatstate.CreateServiceConversation(ctx, db, &servermodels.ServiceConversation{
			OrganizationID: organizationID, ConversationID: conversationID, Source: string(domain.ServiceSourceCerviDirect),
			RequesterSubjectID: sendContext.SubjectID, Audience: string(domain.ServiceAudienceEmployee),
		}); err != nil {
			return nil, err
		}
	}
	return chatstate.OpenServiceSession(ctx, db, organizationID, conversationID, chatstate.OpenServiceSessionInput{
		ID: uuid.NewV7().String(), OpeningMessageID: openingMessageID, OpenedAt: openedAt, AssigneeIdentityID: &sendContext.AgentIdentityID,
	})
}

// scheduleAgentChatInput 把 AI 聊天中的发起人消息交给 AI 员工：属于服务周期时追加到负责 AI 员工的服务输入流，周期由真人负责或在队列中时不调度；不属于服务周期时按普通 AI 聊天调度。
func scheduleAgentChatInput(ctx context.Context, db bun.IDB, organizationID string, sendContext internalMessageContext, session *servermodels.ServiceSession, messageID string, scheduler AgentChatMessageScheduler) error {
	if scheduler == nil {
		return ErrDataInvariant
	}
	if session != nil {
		if _, err := scheduler.ScheduleCustomerAuto(ctx, db, organizationID, sendContext.Conversation.ID, session.ID, messageID); err != nil {
			return fmt.Errorf("schedule service agent input: %w", err)
		}
		return nil
	}
	if sendContext.AgentRevisionID == nil {
		return ErrDataInvariant
	}
	if err := scheduler.Schedule(ctx, db, organizationID, sendContext.Conversation.ID, sendContext.AgentIdentityID, *sendContext.AgentRevisionID, messageID, sendContext.SubjectID, sendContext.AgentInputKind); err != nil {
		return fmt.Errorf("schedule agent input message: %w", err)
	}
	return nil
}

// rejectServiceRequester 在企业身份是服务会话发起人时返回冲突，发起人不能领取、承接或以处理方身份回复自己的请求。
func rejectServiceRequester(ctx context.Context, db bun.IDB, service *servermodels.ServiceConversation, identityID string) error {
	requester, err := db.NewSelect().TableExpr("chat_subjects AS cs").
		Where("cs.organization_id = ? AND cs.id = ? AND cs.kind = ? AND cs.source_id = ?", service.OrganizationID, service.RequesterSubjectID, domain.ChatSubjectKindOrganizationIdentity, identityID).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("check service requester: %w", err)
	}
	if requester {
		return &ConflictError{Reason: ConflictReasonServiceSessionOwnRequest}
	}
	return nil
}
