//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateConversationNotificationSettingsAction 保存当前用户的会话提醒设置。
type UpdateConversationNotificationSettingsAction struct{ db *bun.DB }

// NewUpdateConversationNotificationSettingsAction 创建会话提醒设置操作。
func NewUpdateConversationNotificationSettingsAction(db *bun.DB) *UpdateConversationNotificationSettingsAction {
	return &UpdateConversationNotificationSettingsAction{db: db}
}

// Execute 校验原生会话访问权并保存静音状态。
func (a *UpdateConversationNotificationSettingsAction) Execute(ctx context.Context, identity *servermodels.Identity, conversationID string, muted bool) (ConversationNotificationSettings, error) {
	if !common.ValidUUID(conversationID) {
		return ConversationNotificationSettings{}, &ValidationError{Fields: map[string]ValidationCode{
			"conversationId": ValidationConversationIDInvalid,
		}}
	}
	result := ConversationNotificationSettings{Muted: muted}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		var conversation struct {
			Type   string `bun:"type"`
			Status string `bun:"status"`
		}
		err := tx.NewSelect().
			TableExpr("conversations AS cv").
			ColumnExpr("cv.type, cv.status").
			Join("JOIN conversation_participants AS cp ON cp.organization_id = cv.organization_id AND cp.conversation_id = cv.id AND cp.left_at IS NULL").
			Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id AND cs.kind = ? AND cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).
			Where("cv.organization_id = ?", identity.Organization.ID).
			Where("cv.id = ?", conversationID).
			Where("cv.type IN (?, ?)", domain.ConversationTypeDirect, domain.ConversationTypeGroup).
			Where("cv.status IN (?, ?)", domain.ConversationStatusActive, domain.ConversationStatusArchived).
			Scan(ctx, &conversation)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConversationNotFound
		}
		if err != nil {
			return fmt.Errorf("load conversation notification target: %w", err)
		}
		if domain.ConversationType(conversation.Type) == domain.ConversationTypeDirect && domain.ConversationStatus(conversation.Status) != domain.ConversationStatusActive {
			return ErrConversationNotFound
		}
		state := &servermodels.ConversationUserState{
			OrganizationID: identity.Organization.ID, ConversationID: conversationID,
			UserID: identity.User.ID, Muted: muted,
		}
		if _, err := tx.NewInsert().Model(state).
			Column("organization_id", "conversation_id", "user_id", "muted").
			On("CONFLICT (organization_id, conversation_id, user_id) DO UPDATE").
			Set("muted = EXCLUDED.muted").
			Set("updated_at = now()").
			Exec(ctx); err != nil {
			return fmt.Errorf("save conversation notification settings: %w", err)
		}
		return nil
	})
	if err != nil {
		return ConversationNotificationSettings{}, fmt.Errorf("update conversation notification settings: %w", err)
	}
	return result, nil
}
