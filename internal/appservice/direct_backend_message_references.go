//go:build server

package appservice

import (
	"context"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// ListConversationMessageReferences 认证成员并返回窗口中消息的最新引用状态。
func (b *DirectBackend) ListConversationMessageReferences(ctx context.Context, meta RequestMeta, conversationID string, input ConversationMessageReferenceListInput) (ConversationMessageReferenceList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationMessageReferenceList{}, err
	}
	if !common.ValidUUID(conversationID) {
		return ConversationMessageReferenceList{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	ids := []string{}
	if input.MessageIDs != "" {
		ids = strings.Split(input.MessageIDs, ",")
		for _, id := range ids {
			if !common.ValidUUID(id) {
				return ConversationMessageReferenceList{}, FailedError(meta, cervii18n.ErrorValidationFailed).WithStatus(400)
			}
		}
	}
	messages, err := b.listConversationMessages.ListReferences(ctx, identity, conversationID, ids)
	if err != nil {
		return ConversationMessageReferenceList{}, conversationMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	avatars, err := b.conversationAvatarURLs(ctx, identity, messages)
	if err != nil {
		return ConversationMessageReferenceList{}, err
	}
	states := make([]ConversationMessageReferenceState, 0, len(messages))
	for _, message := range messages {
		mapped := conversationMessageFromAction(message, avatars)
		states = append(states, ConversationMessageReferenceState{MessageID: message.ID, CanReply: mapped.CanReply, ReplyTo: mapped.ReplyTo})
	}
	return ConversationMessageReferenceList{States: states}, nil
}
