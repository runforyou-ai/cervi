//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// ListCustomerMessageDeliveries 读取已登录成员当前窗口的投递状态。
func (b *DirectBackend) ListCustomerMessageDeliveries(ctx context.Context, meta RequestMeta, conversationID string, input CustomerDeliveryListInput) (CustomerDeliveryList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return CustomerDeliveryList{}, err
	}
	if !common.ValidUUID(conversationID) {
		return CustomerDeliveryList{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	ids := []string{}
	if input.MessageIDs != "" {
		ids = strings.Split(input.MessageIDs, ",")
		for _, id := range ids {
			if !common.ValidUUID(id) {
				return CustomerDeliveryList{}, FailedError(meta, cervii18n.ErrorValidationFailed).WithStatus(400)
			}
		}
	}
	rows, err := b.customerDeliveries.List(ctx, identity.Organization.ID, conversationID, ids)
	if err != nil {
		return CustomerDeliveryList{}, customerDeliveryError(meta, err)
	}
	result := make([]CustomerMessageDelivery, 0, len(rows))
	for _, row := range rows {
		result = append(result, CustomerMessageDelivery{CanRetry: row.CanRetry, Paused: row.Paused, ID: row.ID, MessageID: row.MessageID, Status: CustomerDeliveryStatus(row.Status), LastError: row.LastError})
	}
	return CustomerDeliveryList{Deliveries: result}, nil
}

// ResolveCustomerMessageDelivery 认证成员后重新校验投递处理意图。
func (b *DirectBackend) ResolveCustomerMessageDelivery(ctx context.Context, meta RequestMeta, conversationID, deliveryID string, input CustomerDeliveryResolveInput) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if !common.ValidUUID(conversationID) || !common.ValidUUID(deliveryID) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	return customerDeliveryError(meta, b.customerDeliveries.Resolve(ctx, identity, conversationID, deliveryID, domain.CustomerDeliveryResolution(input.Resolution), input.ConfirmDuplicateRisk))
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
