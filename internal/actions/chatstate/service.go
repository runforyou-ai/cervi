//go:build server

package chatstate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ErrDataInvariant 表示聊天持久关系不完整或互相矛盾。
var ErrDataInvariant = errors.New("conversation data invariant violated")

// LockChannelConversation 锁定当前企业的渠道会话。
func LockChannelConversation(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.Conversation, error) {
	conversation := &servermodels.Conversation{}
	err := db.NewSelect().Model(conversation).
		Where("cv.organization_id = ? AND cv.id = ? AND cv.type = ?", organizationID, conversationID, domain.ConversationTypeChannel).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock channel conversation: %w", err)
	}
	return conversation, nil
}

// LockServiceConversation 锁定当前企业承载服务会话的会话。
func LockServiceConversation(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.Conversation, error) {
	conversation := &servermodels.Conversation{}
	err := db.NewSelect().Model(conversation).
		Where("cv.organization_id = ? AND cv.id = ?", organizationID, conversationID).
		Where("EXISTS (SELECT 1 FROM service_conversations AS svc WHERE svc.organization_id = cv.organization_id AND svc.conversation_id = cv.id)").
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock service conversation: %w", err)
	}
	return conversation, nil
}

// LoadServiceConversation 读取会话承载的服务会话。
func LoadServiceConversation(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.ServiceConversation, error) {
	service := &servermodels.ServiceConversation{}
	err := db.NewSelect().Model(service).
		Where("svc.organization_id = ? AND svc.conversation_id = ?", organizationID, conversationID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load service conversation: %w", err)
	}
	return service, nil
}

// LockCurrentServiceSession 在调用方持有会话锁后依次锁定服务会话和当前服务周期。
func LockCurrentServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.ServiceSession, error) {
	service := &servermodels.ServiceConversation{}
	err := db.NewSelect().Model(service).
		Where("svc.organization_id = ? AND svc.conversation_id = ?", organizationID, conversationID).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDataInvariant
	}
	if err != nil {
		return nil, fmt.Errorf("lock service conversation: %w", err)
	}
	if service.CurrentServiceSessionID == nil {
		return nil, ErrDataInvariant
	}
	session := &servermodels.ServiceSession{}
	err = db.NewSelect().Model(session).
		Where("ss.organization_id = ? AND ss.service_conversation_id = ? AND ss.id = ?", organizationID, service.ID, *service.CurrentServiceSessionID).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDataInvariant
	}
	if err != nil {
		return nil, fmt.Errorf("lock current service session: %w", err)
	}
	return session, nil
}

// LockServiceSession 依次锁定承载服务会话的会话、服务会话和当前服务周期。
func LockServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.Conversation, *servermodels.ServiceSession, error) {
	conversation, err := LockServiceConversation(ctx, db, organizationID, conversationID)
	if err != nil {
		return nil, nil, err
	}
	session, err := LockCurrentServiceSession(ctx, db, organizationID, conversationID)
	if err != nil {
		return nil, nil, err
	}
	return conversation, session, nil
}

// CreateServiceConversation 为会话建立服务会话。
func CreateServiceConversation(ctx context.Context, db bun.IDB, service *servermodels.ServiceConversation) error {
	if _, err := db.NewInsert().Model(service).
		Column("organization_id", "conversation_id", "source", "requester_subject_id", "audience").
		Returning("*").
		Exec(ctx); err != nil {
		return fmt.Errorf("create service conversation: %w", err)
	}
	return nil
}

// LockOpenServiceSession 在调用方持有会话锁后锁定服务会话并返回进行中的当前服务周期，当前周期已结束或尚无周期时返回空。
func LockOpenServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.ServiceSession, error) {
	service := &servermodels.ServiceConversation{}
	err := db.NewSelect().Model(service).
		Where("svc.organization_id = ? AND svc.conversation_id = ?", organizationID, conversationID).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDataInvariant
	}
	if err != nil {
		return nil, fmt.Errorf("lock service conversation: %w", err)
	}
	if service.CurrentServiceSessionID == nil {
		return nil, nil
	}
	session, err := LockCurrentServiceSession(ctx, db, organizationID, conversationID)
	if err != nil {
		return nil, err
	}
	switch domain.ServiceSessionStatus(session.Status) {
	case domain.ServiceSessionStatusOpen:
		return session, nil
	case domain.ServiceSessionStatusClosed:
		return nil, nil
	default:
		return nil, ErrDataInvariant
	}
}

// OpenServiceSessionInput 定义开启服务周期的首条消息、开启时间、初始负责方与访客上下文。
type OpenServiceSessionInput struct {
	ID                 string
	OpeningMessageID   string
	OpenedAt           time.Time
	TeamID             *string
	AssigneeIdentityID *string
	VisitorContext     *domain.VisitorContext
}

// OpenServiceSession 在调用方持有会话锁且当前没有进行中周期时开启下一个服务周期，并设为服务会话的当前周期。
func OpenServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID string, input OpenServiceSessionInput) (*servermodels.ServiceSession, error) {
	service := &servermodels.ServiceConversation{}
	err := db.NewSelect().Model(service).
		Where("svc.organization_id = ? AND svc.conversation_id = ?", organizationID, conversationID).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDataInvariant
	}
	if err != nil {
		return nil, fmt.Errorf("lock service conversation: %w", err)
	}
	var sequence int64
	if err := db.NewSelect().Model((*servermodels.ServiceSession)(nil)).
		ColumnExpr("COALESCE(MAX(ss.sequence), 0) + 1").
		Where("ss.organization_id = ? AND ss.service_conversation_id = ?", organizationID, service.ID).
		Scan(ctx, &sequence); err != nil {
		return nil, fmt.Errorf("load next service session sequence: %w", err)
	}
	// 有负责人时记负责时间，无负责人时从首条消息起计入队列。
	var assignedAt, queuedAt *time.Time
	if input.AssigneeIdentityID != nil {
		assignedAt = &input.OpenedAt
	} else {
		queuedAt = &input.OpenedAt
	}
	session := &servermodels.ServiceSession{
		ID: input.ID, OrganizationID: organizationID,
		ConversationID: conversationID, ServiceConversationID: service.ID,
		Sequence: sequence, Status: string(domain.ServiceSessionStatusOpen),
		TeamID: input.TeamID, AssigneeIdentityID: input.AssigneeIdentityID,
		OpeningMessageID: input.OpeningMessageID, LastMessageID: input.OpeningMessageID,
		LastMessageAt: input.OpenedAt,
		AssignedAt:    assignedAt, AssigneeAssignedAt: assignedAt, QueuedAt: queuedAt, StatusChangedAt: input.OpenedAt,
		VisitorContext: input.VisitorContext,
	}
	if _, err := db.NewInsert().Model(session).
		Column("id", "organization_id", "conversation_id", "service_conversation_id", "sequence", "status", "team_id", "assignee_identity_id", "opening_message_id", "last_message_id", "last_message_at", "assigned_at", "assignee_assigned_at", "queued_at", "status_changed_at", "visitor_context").
		Returning("*").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create service session: %w", err)
	}
	if _, err := db.NewUpdate().Model(service).
		Set("current_service_session_id = ?", session.ID).
		Set("updated_at = now()").
		WherePK().
		Where("organization_id = ?", organizationID).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("update current service session: %w", err)
	}
	return session, nil
}

// AppendRequesterStatus 在调用方持有会话锁的事务中为企业成员发起的服务会话写入发起人可见的服务进度事件并返回该事件；渠道来源的服务会话不写入，返回空。
func AppendRequesterStatus(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, session *servermodels.ServiceSession, status domain.ServiceRequestStatus, target *domain.ServiceSessionTarget, closeReason *domain.ServiceSessionCloseReason) (*servermodels.Message, error) {
	var source string
	if err := db.NewSelect().Model((*servermodels.ServiceConversation)(nil)).Column("svc.source").
		Where("svc.organization_id = ? AND svc.id = ?", session.OrganizationID, session.ServiceConversationID).
		Scan(ctx, &source); err != nil {
		return nil, fmt.Errorf("load service conversation source: %w", err)
	}
	if domain.ServiceSource(source) == domain.ServiceSourceChannel {
		return nil, nil
	}
	payload, err := json.Marshal(domain.ServiceStatusChangedEvent{ServiceSessionID: session.ID, Status: status, Target: target, CloseReason: closeReason})
	if err != nil {
		return nil, fmt.Errorf("encode service status event: %w", err)
	}
	eventType := string(domain.ConversationSystemEventServiceStatusChanged)
	message, _, err := AppendMessage(ctx, db, conversation, &servermodels.Message{
		ID: uuid.NewV7().String(), OrganizationID: session.OrganizationID, ConversationID: session.ConversationID,
		ServiceSessionID: &session.ID, Type: string(domain.MessageTypeSystem), Visibility: string(domain.MessageVisibilityRequester),
		SystemEventType: &eventType, SystemEventPayload: payload, OriginatedAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("append service status event: %w", err)
	}
	return message, nil
}
