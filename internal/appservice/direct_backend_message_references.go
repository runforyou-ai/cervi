//go:build server

package appservice

import (
	"context"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ListConversationMessageReferences 返回窗口中消息的最新引用状态。
func (o *directOperations) ListConversationMessageReferences(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input ConversationMessageReferenceListInput) (ConversationMessageReferenceList, error) {
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
	messages, err := o.listConversationMessages.ListReferences(ctx, identity, conversationID, ids)
	if err != nil {
		return ConversationMessageReferenceList{}, conversationMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	avatars, err := o.conversationAvatarURLs(ctx, identity, messages)
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
