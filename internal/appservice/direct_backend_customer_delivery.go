//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ResolveCustomerMessageDelivery 重新校验投递处理意图。
func (o *directOperations) ResolveCustomerMessageDelivery(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID, deliveryID string, input CustomerDeliveryResolveInput) error {
	if !common.ValidUUID(conversationID) || !common.ValidUUID(deliveryID) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	return customerDeliveryError(meta, o.customerDeliveries.Resolve(ctx, identity, conversationID, deliveryID, domain.CustomerDeliveryResolution(input.Resolution), input.ConfirmDuplicateRisk))
}

// customerDeliveryError 转换投递管理错误并保留会话恢复语义。
func customerDeliveryError(meta RequestMeta, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, deliveryaction.ErrUnavailable) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	if errors.Is(err, deliveryaction.ErrConflict) {
		return ConflictError(meta, cervii18n.ErrorCustomerDeliveryConflict, "delivery_state_conflict")
	}
	slog.Warn("客户消息投递操作失败", "error", err)
	return FailedError(meta, cervii18n.ErrorCustomerDeliveryFailed)
}
