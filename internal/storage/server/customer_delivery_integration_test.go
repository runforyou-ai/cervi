//go:build server

package server

import (
	"context"
	"errors"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
	"sync"
	"testing"
	"time"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/telegram"
	models "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

type customerDeliveryFixture struct {
	customerReadFixture
	sender *deliverySender
	worker *deliveryaction.Worker
}
type deliverySender struct {
	mu     sync.Mutex
	bodies []string
	err    error
}

// SendText 记录平台调用并返回可控制的发送结果。
func (s *deliverySender) SendText(_ context.Context, _, recipient, body string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bodies = append(s.bodies, body)
	if recipient != "12345" {
		return 0, errors.New("unexpected recipient")
	}
	return int64(len(s.bodies)), s.err
}

// newCustomerDeliveryFixture 建立已配置的 Telegram 私聊和两个客服身份。
func newCustomerDeliveryFixture(t *testing.T) customerDeliveryFixture {
	t.Helper()
	f := newCustomerReadFixture(t)
	t.Cleanup(func() {
		_, _ = f.db.ExecContext(context.Background(), "DELETE FROM customer_message_deliveries WHERE organization_id = ?", f.owner.Organization.ID)
		_, _ = f.db.ExecContext(context.Background(), "DELETE FROM customer_channel_send_gates WHERE organization_id = ?", f.owner.Organization.ID)
	})
	ctx := context.Background()
	channel, err := channelaction.NewCreateMessageChannelAction(f.db).Execute(ctx, f.owner, channelaction.CreateMessageChannelInput{
		Type: domain.ChannelTypeTelegram, Name: "Telegram 投递测试", DefaultLocale: domain.LocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.channelID = channel.ID
	if _, err := f.db.ExecContext(ctx, "UPDATE telegram_channel_settings SET bot_id = 123, bot_token = '123:token', webhook_secret = 'secret' WHERE channel_id = ?", channel.ID); err != nil {
		t.Fatal(err)
	}
	receiver := channelaction.NewReceiveTelegramWebhookAction(f.db, agentrunaction.NewScheduler(servertask.New(f.db, serverconfig.NATSConfig{})), nil, nil)
	if err := receiver.Execute(ctx, channel.ID, channelaction.TelegramWebhookInput{Secret: "secret", UpdateID: 1, Message: &channelaction.TelegramWebhookMessage{ChatID: 12345, SenderID: 12345, MessageID: 1, DisplayName: "Telegram 客户", Body: "你好", OriginatedAt: time.Now().UTC()}}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.NewSelect().TableExpr("customer_conversations AS cc").ColumnExpr("cc.conversation_id").Join("JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id").Where("cci.channel_id = ?", channel.ID).Scan(ctx, &f.conversationID); err != nil {
		t.Fatal(err)
	}
	sender := &deliverySender{}
	runtime := servertask.New(f.db, serverconfig.NATSConfig{})
	worker := deliveryaction.NewWorker(f.db, sender, runtime)
	if err := runtime.Registry().RegisterJSON(deliveryaction.SendActionName, worker.Execute); err != nil {
		t.Fatal(err)
	}
	return customerDeliveryFixture{f, sender, worker}
}

// send 保存一条客服消息并读取对应投递。
func (f customerDeliveryFixture) send(t *testing.T, body, clientID string) models.CustomerMessageDelivery {
	t.Helper()
	message, err := conversationaction.NewSendCustomerTextMessageAction(f.db, nil).Execute(context.Background(), f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: clientID, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	var delivery models.CustomerMessageDelivery
	if err := f.db.NewSelect().Model(&delivery).Where("message_id = ?", message.ID).Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	return delivery
}

// load 读取投递的持久状态。
func (f customerDeliveryFixture) load(t *testing.T, id string) models.CustomerMessageDelivery {
	t.Helper()
	var delivery models.CustomerMessageDelivery
	if err := f.db.NewSelect().Model(&delivery).Where("id = ?", id).Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	return delivery
}

// execute 运行一次投递并重新读取状态。
func (f customerDeliveryFixture) execute(t *testing.T, id string) models.CustomerMessageDelivery {
	t.Helper()
	if err := f.worker.Execute(context.Background(), deliveryaction.Input{DeliveryID: id}); err != nil {
		t.Fatal(err)
	}
	return f.load(t, id)
}

// TestCustomerDeliveryFIFO 验证幂等入队、并发认领与身份顺序。
func TestCustomerDeliveryFIFO(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	clientID := uuid.NewV7().String()
	first := f.send(t, "第一条", clientID)
	replay := f.send(t, "第一条", clientID)
	if first.ID != replay.ID {
		t.Fatal("duplicate delivery")
	}
	second := f.send(t, "第二条", uuid.NewV7().String())
	if got := f.execute(t, second.ID); got.Status != domain.CustomerDeliveryPending || len(f.sender.bodies) != 0 {
		t.Fatal("queue skipped head")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() { errs <- f.worker.Execute(context.Background(), deliveryaction.Input{DeliveryID: first.ID}) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := f.execute(t, second.ID); got.Status != domain.CustomerDeliverySent || got.ProviderMessageID == nil {
		t.Fatalf("delivery=%+v", got)
	}
	if len(f.sender.bodies) != 2 || f.sender.bodies[0] != "第一条" || f.sender.bodies[1] != "第二条" {
		t.Fatalf("calls=%v", f.sender.bodies)
	}
}

// TestCustomerDeliveryUnknownRecovery 验证未知结果阻塞、人工重试排队与风险确认。
func TestCustomerDeliveryUnknownRecovery(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	ctx := context.Background()
	first := f.send(t, "结果未知", uuid.NewV7().String())
	second := f.send(t, "后续消息", uuid.NewV7().String())
	f.sender.err = &telegram.SendError{Code: "unknown_result"}
	if got := f.execute(t, first.ID); got.Status != domain.CustomerDeliveryUncertain {
		t.Fatal(got.Status)
	}
	f.execute(t, first.ID)
	f.execute(t, second.ID)
	if len(f.sender.bodies) != 1 {
		t.Fatal("unknown result automatically resent")
	}
	if _, err := f.db.ExecContext(ctx, "UPDATE customer_message_deliveries SET uncertain_until = now() - interval '1 second' WHERE id = ?", first.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.execute(t, first.ID); got.Status != domain.CustomerDeliveryNeedsReview {
		t.Fatal(got.Status)
	}
	manager := deliveryaction.NewManager(f.db, nil)
	if err := manager.Resolve(ctx, f.owner, f.conversationID, first.ID, domain.CustomerDeliveryRetry, false); !errors.Is(err, deliveryaction.ErrConflict) {
		t.Fatalf("risk confirmation=%v", err)
	}
	if err := manager.Resolve(ctx, f.owner, f.conversationID, first.ID, domain.CustomerDeliveryRetry, true); err != nil {
		t.Fatal(err)
	}
	if f.load(t, first.ID).Position <= second.Position {
		t.Fatal("retry not at tail")
	}
	f.sender.err = nil
	f.execute(t, first.ID)
	if len(f.sender.bodies) != 1 {
		t.Fatal("retry skipped queued message")
	}
	f.execute(t, second.ID)
	f.execute(t, first.ID)
	if len(f.sender.bodies) != 3 || f.sender.bodies[1] != "后续消息" {
		t.Fatal(f.sender.bodies)
	}
}

// TestCustomerDeliveryLeaseAndLifecycle 验证过期认领、渠道停用和机器人变化。
func TestCustomerDeliveryLeaseAndLifecycle(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	ctx := context.Background()
	first := f.send(t, "中断消息", uuid.NewV7().String())
	if _, err := f.db.ExecContext(ctx, "UPDATE customer_message_deliveries SET status = 'sending', lease_worker = ?, lease_expires_at = now() - interval '1 second' WHERE id = ?", uuid.NewV7().String(), first.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.execute(t, first.ID); got.Status != domain.CustomerDeliveryUncertain || len(f.sender.bodies) != 0 {
		t.Fatal("expired sending was resent")
	}
	if _, err := f.db.ExecContext(ctx, "UPDATE customer_message_deliveries SET uncertain_until = now() - interval '1 second' WHERE id = ?", first.ID); err != nil {
		t.Fatal(err)
	}
	f.execute(t, first.ID)
	second := f.send(t, "等待恢复", uuid.NewV7().String())
	if _, err := f.db.ExecContext(ctx, "UPDATE channels SET enabled = false WHERE id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	if got := f.execute(t, second.ID); got.Status != domain.CustomerDeliveryPending || got.LastError != "channel_disabled" {
		t.Fatalf("paused=%+v", got)
	}
	if _, err := f.db.ExecContext(ctx, "UPDATE channels SET enabled = true WHERE id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	if got := f.execute(t, second.ID); got.Status != domain.CustomerDeliverySent {
		t.Fatal(got.Status)
	}
	third := f.send(t, "旧机器人消息", uuid.NewV7().String())
	if _, err := f.db.ExecContext(ctx, "UPDATE telegram_channel_settings SET bot_id = 456 WHERE channel_id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	if got := f.execute(t, third.ID); got.Status != domain.CustomerDeliveryFailed || got.LastError != "bot_changed" || len(f.sender.bodies) != 1 {
		t.Fatalf("changed bot=%+v", got)
	}
}

// TestCustomerDeliveryRateLimitAndIsolation 验证渠道等待、永久拒绝和企业隔离。
func TestCustomerDeliveryRateLimitAndIsolation(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	ctx := context.Background()
	first := f.send(t, "限流消息", uuid.NewV7().String())
	f.sender.err = &telegram.SendError{Code: "rate_limited", RetryAfter: time.Minute}
	if got := f.execute(t, first.ID); got.Status != domain.CustomerDeliveryRetryWait {
		t.Fatal(got.Status)
	}
	f.execute(t, first.ID)
	if len(f.sender.bodies) != 1 {
		t.Fatal("ignored rate limit")
	}
	if _, err := f.db.ExecContext(ctx, "UPDATE customer_message_deliveries SET available_at = now() - interval '1 second' WHERE id = ?", first.ID); err != nil {
		t.Fatal(err)
	}
	f.execute(t, first.ID)
	if len(f.sender.bodies) != 1 {
		t.Fatal("ignored channel gate")
	}
	if _, err := f.db.ExecContext(ctx, "UPDATE customer_channel_send_gates SET flood_wait_until = now() - interval '1 second' WHERE channel_id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	f.sender.err = &telegram.SendError{Code: "recipient_unavailable"}
	if got := f.execute(t, first.ID); got.Status != domain.CustomerDeliveryFailed {
		t.Fatal(got.Status)
	}
	manager := deliveryaction.NewManager(f.db, nil)
	if _, err := manager.List(ctx, uuid.NewV7().String(), f.conversationID, []string{first.MessageID}); !errors.Is(err, deliveryaction.ErrUnavailable) {
		t.Fatal("cross-organization delivery visible")
	}
	f.sender.err = nil
	if err := manager.Resolve(ctx, f.owner, f.conversationID, first.ID, domain.CustomerDeliveryRetry, false); err != nil {
		t.Fatal(err)
	}
	if got := f.execute(t, first.ID); got.Status != domain.CustomerDeliverySent {
		t.Fatal(got.Status)
	}
}

// TestCustomerDeliveryScanAndManualConfirmation 验证没有快速唤醒时扫描恢复以及人工确认。
func TestCustomerDeliveryScanAndManualConfirmation(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	ctx := context.Background()
	first := f.send(t, "扫描恢复", uuid.NewV7().String())
	// 用数据库时间显式设置到期，避免宿主和容器的毫秒级时钟差。
	if _, err := f.db.ExecContext(ctx, "UPDATE customer_message_deliveries SET available_at = now() - interval '1 second' WHERE id = ?", first.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.worker.Scan(ctx, struct{}{}); err != nil {
		t.Fatal(err)
	}
	// 扫描只创建唤醒，模拟 Worker 消费后才调用平台。
	if len(f.sender.bodies) != 0 {
		t.Fatal("scanner called Telegram")
	}
	if exists, err := f.db.NewSelect().TableExpr("task_runs").Where("idempotency_key = ?", "cdeliv-item:"+first.ID).Exists(ctx); err != nil || !exists {
		t.Fatalf("scan wakeup: %v %v", exists, err)
	}
	if got := f.execute(t, first.ID); got.Status != domain.CustomerDeliverySent {
		t.Fatal(got.Status)
	}
	second := f.send(t, "人工确认", uuid.NewV7().String())
	if _, err := f.db.ExecContext(ctx, "UPDATE customer_message_deliveries SET status = 'needs_review', last_error = 'unknown_result' WHERE id = ?", second.ID); err != nil {
		t.Fatal(err)
	}
	manager := deliveryaction.NewManager(f.db, nil)
	if err := manager.Resolve(ctx, f.owner, f.conversationID, second.ID, domain.CustomerDeliveryConfirmSent, false); err != nil {
		t.Fatal(err)
	}
	got := f.load(t, second.ID)
	if got.Status != domain.CustomerDeliverySent || got.ProviderMessageID != nil || got.LastError != "manually_confirmed" {
		t.Fatalf("manual confirmation=%+v", got)
	}
	if err := manager.Resolve(ctx, f.owner, f.conversationID, second.ID, domain.CustomerDeliveryRetry, true); !errors.Is(err, deliveryaction.ErrConflict) {
		t.Fatal("stale operation accepted")
	}
	other := newCustomerDeliveryFixture(t)
	if err := manager.Resolve(ctx, other.owner, f.conversationID, second.ID, domain.CustomerDeliveryConfirmFailed, false); !errors.Is(err, deliveryaction.ErrUnavailable) {
		t.Fatal("cross-organization operation accepted")
	}
}

type failingDeliveryEnqueuer struct {
	observedAtomicRows bool
	inner              servertask.TxEnqueuer
	taskID             string
}

// EnqueueIn 检查业务行与投递行已进入同一事务，再模拟唤醒写入失败。
func (e *failingDeliveryEnqueuer) EnqueueIn(ctx context.Context, tx bun.IDB, action string, input any, options servertask.EnqueueOptions) (string, error) {
	id := input.(deliveryaction.Input).DeliveryID
	var err error
	e.observedAtomicRows, err = tx.NewSelect().TableExpr("customer_message_deliveries AS d").Join("JOIN messages AS m ON m.id = d.message_id AND m.organization_id = d.organization_id").Join("JOIN conversations cv ON cv.id = m.conversation_id AND cv.last_message_id = m.id").Join("JOIN service_sessions ss ON ss.id = m.service_session_id AND ss.last_message_id = m.id").Where("d.id = ?", id).Exists(ctx)
	if err != nil {
		return "", err
	}
	e.taskID, err = e.inner.EnqueueIn(ctx, tx, action, input, options)
	if err != nil {
		return "", err
	}
	return "", errors.New("enqueue failed")
}

// TestCustomerDeliveryAtomicEnqueue 验证唤醒失败回滚消息与投递，成功时提交可靠任务。
func TestCustomerDeliveryAtomicEnqueue(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	ctx := context.Background()
	input := conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "必须原子提交"}
	runtime := servertask.New(f.db, serverconfig.NATSConfig{})
	if err := runtime.Registry().RegisterJSON(deliveryaction.SendActionName, f.worker.Execute); err != nil {
		t.Fatal(err)
	}
	before := &models.Conversation{ID: f.conversationID}
	if err := f.db.NewSelect().Model(before).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	failing := &failingDeliveryEnqueuer{inner: runtime}
	if _, err := conversationaction.NewSendCustomerTextMessageAction(f.db, failing).Execute(ctx, f.owner, input); err == nil || !failing.observedAtomicRows {
		t.Fatalf("atomic rows=%v err=%v", failing.observedAtomicRows, err)
	}
	if exists, err := f.db.NewSelect().TableExpr("customer_message_deliveries").Where("conversation_id = ?", f.conversationID).Exists(ctx); err != nil || exists {
		t.Fatalf("delivery survived rollback: %v %v", exists, err)
	}
	if exists, err := f.db.NewSelect().TableExpr("messages").Where("conversation_id = ? AND body = ?", f.conversationID, input.Body).Exists(ctx); err != nil || exists {
		t.Fatalf("message survived rollback: %v %v", exists, err)
	}
	if failing.taskID == "" {
		t.Fatal("task was not written before rollback")
	}
	for table, column := range map[string]string{"task_runs": "id", "task_outbox": "task_run_id"} {
		count, err := f.db.NewSelect().TableExpr(table).Where("? = ?", bun.Ident(column), failing.taskID).Count(ctx)
		if err != nil || count != 0 {
			t.Fatalf("rollback %s rows=%d err=%v", table, count, err)
		}
	}
	after := &models.Conversation{ID: f.conversationID}
	if err := f.db.NewSelect().Model(after).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if after.LastMessageID == nil || *after.LastMessageID != *before.LastMessageID || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("summary survived rollback: before=%+v after=%+v", before, after)
	}
	assertCustomerLockSummary(t, ctx, f.db, f.conversationID)
	message, err := conversationaction.NewSendCustomerTextMessageAction(f.db, runtime).Execute(ctx, f.owner, input)
	if err != nil {
		t.Fatal(err)
	}
	exists, err := f.db.NewSelect().TableExpr("customer_message_deliveries AS d").Join("JOIN task_runs AS tr ON tr.idempotency_key = 'cdeliv-item:' || d.id::text").Join("JOIN task_outbox AS tob ON tob.task_run_id = tr.id").Where("d.message_id = ?", message.ID).Exists(ctx)
	if err != nil || !exists {
		t.Fatalf("missing atomic wakeup: %v %v", exists, err)
	}
}

// TestCustomerDeliveryBotMessageNamespace 验证换 Bot 后相同平台消息编号不会冲突。
func TestCustomerDeliveryBotMessageNamespace(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	ctx := context.Background()
	first := f.send(t, "旧机器人回复", uuid.NewV7().String())
	f.execute(t, first.ID)
	if _, err := f.db.ExecContext(ctx, "UPDATE telegram_channel_settings SET bot_id = 456, bot_token = '456:token' WHERE channel_id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	receiver := channelaction.NewReceiveTelegramWebhookAction(f.db, agentrunaction.NewScheduler(servertask.New(f.db, serverconfig.NATSConfig{})), nil, nil)
	if err := receiver.Execute(ctx, f.channelID, channelaction.TelegramWebhookInput{Secret: "secret", UpdateID: 1, Message: &channelaction.TelegramWebhookMessage{ChatID: 12345, SenderID: 12345, MessageID: 1, DisplayName: "Telegram 客户", Body: "新机器人首条消息", OriginatedAt: time.Now().UTC()}}); err != nil {
		t.Fatal(err)
	}
	exists, err := f.db.NewSelect().TableExpr("messages").Where("conversation_id = ? AND body = ?", f.conversationID, "新机器人首条消息").Exists(ctx)
	if err != nil || !exists {
		t.Fatalf("new bot inbound lost: %v %v", exists, err)
	}
	// 新机器人可以返回与旧机器人相同的平台消息编号。
	f.sender.bodies = nil
	second := f.send(t, "新机器人回复", uuid.NewV7().String())
	got := f.execute(t, second.ID)
	if got.Status != domain.CustomerDeliverySent || got.ProviderMessageID == nil || *got.ProviderMessageID != 1 {
		t.Fatalf("new bot result=%+v", got)
	}
}

// TestCustomerDeliveryCurrentCapabilities 验证暂停状态和重试入口随当前渠道变化。
func TestCustomerDeliveryCurrentCapabilities(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	ctx := context.Background()
	first := f.send(t, "待确认消息", uuid.NewV7().String())
	if _, err := f.db.ExecContext(ctx, "UPDATE customer_message_deliveries SET status = 'needs_review', last_error = 'unknown_result' WHERE id = ?", first.ID); err != nil {
		t.Fatal(err)
	}
	second := f.send(t, "等待发送", uuid.NewV7().String())
	manager := deliveryaction.NewManager(f.db, nil)
	rows, err := manager.List(ctx, f.owner.Organization.ID, f.conversationID, []string{first.MessageID, second.MessageID})
	if err != nil || len(rows) != 2 {
		t.Fatalf("list=%+v err=%v", rows, err)
	}
	if !rows[0].CanRetry || rows[1].Paused {
		t.Fatal("initial capabilities incorrect")
	}
	if _, err := f.db.ExecContext(ctx, "UPDATE channels SET enabled = false WHERE id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	rows, err = manager.List(ctx, f.owner.Organization.ID, f.conversationID, []string{first.MessageID, second.MessageID})
	if err != nil || rows[0].CanRetry || !rows[1].Paused {
		t.Fatalf("disabled capabilities=%+v err=%v", rows, err)
	}
	if _, err := f.db.ExecContext(ctx, "UPDATE channels SET enabled = true WHERE id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, "UPDATE telegram_channel_settings SET bot_id = 456 WHERE channel_id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	rows, err = manager.List(ctx, f.owner.Organization.ID, f.conversationID, []string{first.MessageID})
	if err != nil || rows[0].CanRetry || rows[0].Status != domain.CustomerDeliveryNeedsReview {
		t.Fatalf("changed bot capabilities=%+v err=%v", rows, err)
	}
}
