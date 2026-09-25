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

// GenerateServiceReplySuggestions 使用 AI 员工为服务会话生成回复候选。
func (o *directOperations) GenerateServiceReplySuggestions(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input ServiceReplySuggestionsInput) (ServiceReplySuggestions, error) {
	candidates, err := o.serviceReplySuggestions.Execute(ctx, identity, agentrunaction.ServiceReplySuggestionsInput{
		ConversationID: conversationID, AgentIdentityID: input.AgentIdentityID,
		Mode: domain.ServiceReplyMode(input.Mode), Tone: domain.ServiceReplyTone(input.Tone),
		Draft: input.Draft, ReplyToMessageID: input.ReplyToMessageID, Language: input.Language,
	})
	if err == nil {
		return ServiceReplySuggestions{Candidates: candidates}, nil
	}
	if ctx.Err() != nil {
		return ServiceReplySuggestions{}, ctx.Err()
	}
	switch {
	case errors.Is(err, conversationaction.ErrConversationNotFound):
		return ServiceReplySuggestions{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	case errors.Is(err, agentrunaction.ErrAgentUnavailable):
		return ServiceReplySuggestions{}, NotFoundError(meta, cervii18n.ErrorAgentUnavailable)
	case errors.Is(err, agentrunaction.ErrCustomerReplyGenerationFailed):
		return ServiceReplySuggestions{}, FailedError(meta, cervii18n.ErrorCustomerReplySuggestFailed)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return ServiceReplySuggestions{}, InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, serviceReplySuggestionsValidationKeys))
	}
	if conflictError, ok := errors.AsType[*conversationaction.ConflictError](err); ok {
		return ServiceReplySuggestions{}, ConflictError(meta, customerReplyConflictMessageKey(conflictError.Reason), conflictError.Reason)
	}
	slog.Warn("读取客户回复候选资料失败", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "error", err)
	return ServiceReplySuggestions{}, FailedError(meta, cervii18n.ErrorCustomerReplySuggestFailed)
}

// ListServiceReplyAgents 返回可用于 AI 写回复的 AI 员工。
func (o *directOperations) ListServiceReplyAgents(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (ServiceReplyAgentList, error) {
	agents, err := o.listServiceReplyAgents.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return ServiceReplyAgentList{}, ctx.Err()
		}
		slog.Warn("读取 AI 写回复可用员工失败", "organization_id", identity.Organization.ID, "error", err)
		return ServiceReplyAgentList{}, FailedError(meta, cervii18n.ErrorAgentListFailed)
	}
	output := make([]ServiceReplyAgent, 0, len(agents))
	for _, agent := range agents {
		output = append(output, ServiceReplyAgent{IdentityID: agent.IdentityID, DisplayName: agent.DisplayName})
	}
	return ServiceReplyAgentList{Agents: output}, nil
}

var serviceReplySuggestionsValidationKeys = map[common.FieldCode]cervii18n.Key{
	conversationaction.ValidationConversationIDInvalid:    cervii18n.FieldConversationIDInvalid,
	agentrunaction.ValidationAgentIdentityIDInvalid:       cervii18n.FieldAgentIdentityIDInvalid,
	agentrunaction.ValidationServiceReplyModeInvalid:      cervii18n.FieldServiceReplyModeInvalid,
	agentrunaction.ValidationServiceReplyToneInvalid:      cervii18n.FieldServiceReplyToneInvalid,
	agentrunaction.ValidationCustomerReplyLanguageInvalid: cervii18n.FieldLocaleInvalid,
	conversationaction.ValidationReplyToMessageIDInvalid:  cervii18n.FieldReplyToMessageIDInvalid,
	conversationaction.ValidationBodyRequired:             cervii18n.FieldCustomerReplyDraftRequired,
	conversationaction.ValidationBodyTooLong:              cervii18n.FieldMessageBodyTooLong,
}
