//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// GenerateCustomerReplySuggestions 使用 AI 员工为客户会话生成对客回复候选。
func (o *directOperations) GenerateCustomerReplySuggestions(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input CustomerReplySuggestionsInput) (CustomerReplySuggestions, error) {
	candidates, err := o.customerReplySuggestions.Execute(ctx, identity, agentrunaction.CustomerReplySuggestionsInput{
		ConversationID: conversationID, AgentIdentityID: input.AgentIdentityID,
		Mode: domain.CustomerReplyMode(input.Mode), Tone: domain.CustomerReplyTone(input.Tone),
		Draft: input.Draft, ReplyToMessageID: input.ReplyToMessageID, Language: input.Language,
	})
	if err == nil {
		return CustomerReplySuggestions{Candidates: candidates}, nil
	}
	if ctx.Err() != nil {
		return CustomerReplySuggestions{}, ctx.Err()
	}
	switch {
	case errors.Is(err, conversationaction.ErrConversationNotFound):
		return CustomerReplySuggestions{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	case errors.Is(err, agentrunaction.ErrAgentUnavailable):
		return CustomerReplySuggestions{}, NotFoundError(meta, cervii18n.ErrorAgentUnavailable)
	case errors.Is(err, agentrunaction.ErrCustomerReplyGenerationFailed):
		return CustomerReplySuggestions{}, FailedError(meta, cervii18n.ErrorCustomerReplySuggestFailed)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return CustomerReplySuggestions{}, InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, customerReplySuggestionsValidationKeys))
	}
	if conflictError, ok := errors.AsType[*conversationaction.ConflictError](err); ok {
		return CustomerReplySuggestions{}, ConflictError(meta, customerReplyConflictMessageKey(conflictError.Reason), conflictError.Reason)
	}
	slog.Warn("读取客户回复候选资料失败", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "error", err)
	return CustomerReplySuggestions{}, FailedError(meta, cervii18n.ErrorCustomerReplySuggestFailed)
}

// ListCustomerReplyAgents 返回可用于 AI 写回复的 AI 员工。
func (o *directOperations) ListCustomerReplyAgents(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (CustomerReplyAgentList, error) {
	agents, err := o.listCustomerReplyAgents.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return CustomerReplyAgentList{}, ctx.Err()
		}
		slog.Warn("读取 AI 写回复可用员工失败", "organization_id", identity.Organization.ID, "error", err)
		return CustomerReplyAgentList{}, FailedError(meta, cervii18n.ErrorAgentListFailed)
	}
	output := make([]CustomerReplyAgent, 0, len(agents))
	for _, agent := range agents {
		output = append(output, CustomerReplyAgent{IdentityID: agent.IdentityID, DisplayName: agent.DisplayName})
	}
	return CustomerReplyAgentList{Agents: output}, nil
}

var customerReplySuggestionsValidationKeys = map[common.FieldCode]cervii18n.Key{
	conversationaction.ValidationConversationIDInvalid:    cervii18n.FieldConversationIDInvalid,
	agentrunaction.ValidationAgentIdentityIDInvalid:       cervii18n.FieldAgentIdentityIDInvalid,
	agentrunaction.ValidationCustomerReplyModeInvalid:     cervii18n.FieldCustomerReplyModeInvalid,
	agentrunaction.ValidationCustomerReplyToneInvalid:     cervii18n.FieldCustomerReplyToneInvalid,
	agentrunaction.ValidationCustomerReplyLanguageInvalid: cervii18n.FieldLocaleInvalid,
	conversationaction.ValidationReplyToMessageIDInvalid:  cervii18n.FieldReplyToMessageIDInvalid,
	conversationaction.ValidationBodyRequired:             cervii18n.FieldCustomerReplyDraftRequired,
	conversationaction.ValidationBodyTooLong:              cervii18n.FieldMessageBodyTooLong,
}
