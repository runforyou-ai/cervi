//go:build server

package chatstate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ErrDataInvariant 表示聊天持久关系不完整或互相矛盾。
var ErrDataInvariant = errors.New("conversation data invariant violated")

// LockCustomerConversation 锁定当前企业的客户会话。
func LockCustomerConversation(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.Conversation, error) {
	conversation := &servermodels.Conversation{}
	err := db.NewSelect().Model(conversation).
		Where("cv.organization_id = ? AND cv.id = ? AND cv.type = ?", organizationID, conversationID, domain.ConversationTypeCustomer).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock customer conversation: %w", err)
	}
	return conversation, nil
}

// LockCurrentServiceSession 在调用方持有会话锁后依次锁定客户扩展和当前客服周期。
func LockCurrentServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.ServiceSession, error) {
	customer := &servermodels.CustomerConversation{}
	err := db.NewSelect().Model(customer).
		Where("cc.organization_id = ? AND cc.conversation_id = ?", organizationID, conversationID).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDataInvariant
	}
	if err != nil {
		return nil, fmt.Errorf("lock customer conversation: %w", err)
	}
	if customer.CurrentServiceSessionID == nil {
		return nil, ErrDataInvariant
	}
	session := &servermodels.ServiceSession{}
	err = db.NewSelect().Model(session).
		Where("ss.organization_id = ? AND ss.conversation_id = ? AND ss.id = ?", organizationID, conversationID, *customer.CurrentServiceSessionID).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDataInvariant
	}
	if err != nil {
		return nil, fmt.Errorf("lock current service session: %w", err)
	}
	if session.ContactChannelIdentityID != customer.ContactChannelIdentityID {
		return nil, ErrDataInvariant
	}
	return session, nil
}

// LockCustomerServiceSession 依次锁定已有客户会话、客户扩展和当前客服周期。
func LockCustomerServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.ServiceSession, error) {
	if _, err := LockCustomerConversation(ctx, db, organizationID, conversationID); err != nil {
		return nil, err
	}
	return LockCurrentServiceSession(ctx, db, organizationID, conversationID)
}
