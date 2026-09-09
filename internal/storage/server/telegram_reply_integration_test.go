//go:build server

package server

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/telegram"
	models "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
)

// receiveReply 通过真实入站事务保存引用消息。
func (f customerDeliveryFixture) receiveReply(t *testing.T, id int64, body string, reply *channelaction.TelegramWebhookReply) {
	t.Helper()
	receiver := channelaction.NewReceiveTelegramWebhookAction(f.db, agentrunaction.NewScheduler(servertask.New(f.db, serverconfig.NATSConfig{})), nil, nil)
	if err := receiver.Execute(context.Background(), f.channelID, channelaction.TelegramWebhookInput{Secret: "secret", UpdateID: id,
		Message: &channelaction.TelegramWebhookMessage{ChatID: 12345, SenderID: 12345, MessageID: id, DisplayName: "Telegram 客户", Body: body, OriginatedAt: time.Now().UTC(), Reply: reply}}); err != nil {
		t.Fatal(err)
	}
}

// replyHistory 读取带引用关系和可用状态的真实会话窗口。
func (f customerDeliveryFixture) replyHistory(t *testing.T) []conversationaction.ConversationMessage {
	t.Helper()
	history, err := conversationaction.NewListConversationMessagesQuery(f.db).Execute(context.Background(), f.owner, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID})
	if err != nil {
		t.Fatal(err)
	}
	return history.Messages
}

// sendReply 保存引用回复并读取其固定投递目标。
func (f customerDeliveryFixture) sendReply(t *testing.T, target string) models.CustomerMessageDelivery {
	t.Helper()
	msg, err := conversationaction.NewSendCustomerTextMessageAction(f.db, nil).Execute(context.Background(), f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "引用回答", ReplyToMessageID: target})
	if err != nil {
		t.Fatal(err)
	}
	var delivery models.CustomerMessageDelivery
	if err := f.db.NewSelect().Model(&delivery).Where("message_id = ?", msg.ID).Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	return delivery
}

// TestTelegramReplyRoundTrip 验证引用客户和客服消息、平台参数以及幂等入站。
func TestTelegramReplyRoundTrip(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	original := f.replyHistory(t)[0]
	if original.ReplyUnavailable {
		t.Fatal("inbound message cannot be replied to")
	}
	delivery := f.sendReply(t, original.ID)
	if delivery.ReplyProviderMessageID == nil || *delivery.ReplyProviderMessageID != "1" {
		t.Fatalf("target=%+v", delivery)
	}
	states, err := conversationaction.NewListConversationMessagesQuery(f.db).ListReferences(context.Background(), f.owner, f.conversationID, []string{delivery.MessageID})
	if err != nil || len(states) != 1 || !states[0].ReplyUnavailable {
		t.Fatalf("pending state=%+v err=%v", states, err)
	}
	sent := f.execute(t, delivery.ID)
	states, err = conversationaction.NewListConversationMessagesQuery(f.db).ListReferences(context.Background(), f.owner, f.conversationID, []string{delivery.MessageID})
	if err != nil || len(states) != 1 || states[0].ReplyUnavailable {
		t.Fatalf("sent state=%+v err=%v", states, err)
	}
	if sent.Status != domain.CustomerDeliverySent || f.sender.replies[0] == nil || *f.sender.replies[0] != "1" {
		t.Fatalf("sent=%+v", sent)
	}
	reply := &channelaction.TelegramWebhookReply{MessageID: *sent.ProviderMessageID, Body: "引用回答", SenderName: "Bot", SenderIsBot: true}
	f.receiveReply(t, 2, "我引用客服的回答", reply)
	f.receiveReply(t, 2, "我引用客服的回答", reply)
	history := f.replyHistory(t)
	if len(history) != 3 || history[2].ReplyTo == nil || history[2].ReplyTo.ID != delivery.MessageID || history[2].ReplyTo.Body != "引用回答" {
		t.Fatalf("history=%+v", history)
	}
	next := f.sendReply(t, delivery.MessageID)
	f.execute(t, next.ID)
	if next.ReplyProviderMessageID == nil || *next.ReplyProviderMessageID != strconv.FormatInt(*sent.ProviderMessageID, 10) {
		t.Fatal("outbound target lost")
	}
}

// TestTelegramReplyLateMapping 验证乱序原消息、迟到回执、引用快照和重放关联稳定。
func TestTelegramReplyLateMapping(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	reply := &channelaction.TelegramWebhookReply{MessageID: 9, Body: "较早原文", SenderName: "外部客户"}
	f.receiveReply(t, 10, "原消息后到", reply)
	history := f.replyHistory(t)
	if r := history[1].ReplyTo; r == nil || r.ID != "" || r.Body != reply.Body || r.ExternalSenderName != reply.SenderName {
		t.Fatalf("snapshot=%+v", r)
	}
	f.receiveReply(t, 9, "较早原文", nil)
	f.receiveReply(t, 10, "原消息后到", reply)
	history = f.replyHistory(t)
	if len(history) != 3 || history[1].ReplyTo == nil || history[1].ReplyTo.ID != history[2].ID {
		t.Fatalf("late original=%+v", history)
	}
	delivery := f.send(t, "稍后返回的客服回执", uuid.NewV7().String())
	f.receiveReply(t, 11, "回复先于回执", &channelaction.TelegramWebhookReply{MessageID: 1001, Body: "稍后返回的客服回执", SenderName: "Bot", SenderIsBot: true})
	f.execute(t, delivery.ID)
	history = f.replyHistory(t)
	if history[len(history)-1].ReplyTo.ID != delivery.MessageID {
		t.Fatal("late receipt not linked")
	}
	// 核验重放时保留首次接收的平台引用信息。
	f.receiveReply(t, 10, "原消息后到", &channelaction.TelegramWebhookReply{MessageID: 1})
	history = f.replyHistory(t)
	if history[1].ReplyTo.ID != history[2].ID {
		t.Fatal("conflicting replay changed target")
	}
}

// TestTelegramReplyTargetBoundaries 验证无回执、跨会话、删除与换机器人时禁止引用。
func TestTelegramReplyTargetBoundaries(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	other := newCustomerDeliveryFixture(t)
	original := f.replyHistory(t)[0]
	pending := f.send(t, "尚未投递", uuid.NewV7().String())
	for _, target := range []string{pending.MessageID, other.replyHistory(t)[0].ID, uuid.NewV7().String()} {
		_, err := conversationaction.NewSendCustomerTextMessageAction(f.db, nil).Execute(context.Background(), f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "无效引用", ReplyToMessageID: target})
		var conflict *conversationaction.ConflictError
		if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonReplyTargetInvalid {
			t.Fatalf("target=%s err=%v", target, err)
		}
	}
	if _, err := f.db.ExecContext(context.Background(), "UPDATE customer_message_deliveries SET status = 'needs_review' WHERE id = ?", pending.ID); err != nil {
		t.Fatal(err)
	}
	if err := deliveryaction.NewManager(f.db, nil).Resolve(context.Background(), f.owner, f.conversationID, pending.ID, domain.CustomerDeliveryConfirmSent, false); err != nil {
		t.Fatal(err)
	}
	if !f.replyHistory(t)[1].ReplyUnavailable {
		t.Fatal("manual confirmation invented platform identity")
	}
	f.receiveReply(t, 2, "引用后删除", &channelaction.TelegramWebhookReply{MessageID: 1, Body: original.Body})
	if _, err := f.db.ExecContext(context.Background(), "UPDATE messages SET deleted_at = now() WHERE id = ?", original.ID); err != nil {
		t.Fatal(err)
	}
	history := f.replyHistory(t)
	if r := history[len(history)-1].ReplyTo; r == nil || !r.Deleted || r.Body != "" || r.ExternalSenderName != "" {
		t.Fatalf("deleted=%+v", r)
	}
	if _, err := f.db.ExecContext(context.Background(), "UPDATE telegram_channel_settings SET bot_id = 456 WHERE channel_id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	for _, message := range f.replyHistory(t) {
		if !message.ReplyUnavailable {
			t.Fatal("old bot remains replyable")
		}
	}
	// 核验引用消息的平台机器人归属。
	f.receiveReply(t, 3, "新机器人的引用", &channelaction.TelegramWebhookReply{MessageID: 2, Body: "新机器人原文"})
	history = f.replyHistory(t)
	if history[len(history)-1].ReplyTo.ID != "" {
		t.Fatal("cross-bot reference")
	}
}

// TestTelegramReplyDeliveryFailures 验证限流重试保留引用及平台拒绝时的发送失败状态。
func TestTelegramReplyDeliveryFailures(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	delivery := f.sendReply(t, f.replyHistory(t)[0].ID)
	f.sender.err = &telegram.SendError{Code: "rate_limited", RetryAfter: time.Millisecond}
	if got := f.execute(t, delivery.ID); got.Status != domain.CustomerDeliveryRetryWait {
		t.Fatalf("delivery=%+v", got)
	}
	if _, err := f.db.ExecContext(context.Background(), "UPDATE customer_message_deliveries SET available_at = now() WHERE id = ?", delivery.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(context.Background(), "UPDATE customer_channel_send_gates SET flood_wait_until = now() WHERE channel_id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	f.sender.err = &telegram.SendError{Code: "message_rejected"}
	if got := f.execute(t, delivery.ID); got.Status != domain.CustomerDeliveryFailed {
		t.Fatalf("delivery=%+v", got)
	}
	if len(f.sender.replies) != 2 || *f.sender.replies[0] != "1" || *f.sender.replies[1] != "1" {
		t.Fatal("reply target changed")
	}
	f.execute(t, delivery.ID)
	if len(f.sender.replies) != 2 {
		t.Fatal("rejected reply resent")
	}
}
