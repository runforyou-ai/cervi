//go:build server

package customernotify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	translationaction "github.com/runforyou-ai/cervi/internal/actions/translation"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/mail"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// NotifyActionName 在客服回复后检查访客是否已读，未读时发送邮件通知。
const NotifyActionName = "customer_notify.email"

const (
	// notifyDelay 是客服回复后等待访客阅读的时长。
	notifyDelay = 3 * time.Minute
	// notifyMaxAttempts 是邮件通知任务的最大尝试次数，发信失败时按任务退避重试。
	notifyMaxAttempts = 5
	// ResumeTokenTTL 是邮件中回访令牌的有效期。
	ResumeTokenTTL = 30 * 24 * time.Hour
	// websiteVisitorExternalIDPrefix 是网站匿名访客渠道外部编号的前缀，其后为访客令牌。
	websiteVisitorExternalIDPrefix = "web-session:"
)

// NotifyInput 定义一次邮件通知检查：触发回复所在会话与其消息序号。
type NotifyInput struct {
	OrganizationID string `json:"organizationId"`
	ConversationID string `json:"conversationId"`
	MessageSeq     int64  `json:"messageSeq"`
}

// Enqueue 在真人对客回复的事务中登记延迟的邮件通知检查。
func Enqueue(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, input NotifyInput) error {
	if _, err := enqueuer.EnqueueIn(ctx, db, NotifyActionName, input, servertask.EnqueueOptions{
		MaxAttempts: notifyMaxAttempts, AvailableAt: time.Now().Add(notifyDelay),
	}); err != nil {
		return fmt.Errorf("enqueue customer email notification: %w", err)
	}
	return nil
}

// Worker 发送客服回复的邮件通知。
type Worker struct {
	db     *bun.DB
	sender Sender
	scheme string
}

// NewWorker 创建邮件通知 Worker；scheme 是企业访问入口的协议，sender 为空时任务直接结束。
func NewWorker(db *bun.DB, sender Sender, scheme string) *Worker {
	return &Worker{db: db, sender: sender, scheme: scheme}
}

// pendingReply 是一条等待邮件通知的真人对客回复。
type pendingReply struct {
	MessageSeq       int64   `bun:"message_seq"`
	ServiceSessionID string  `bun:"service_session_id"`
	Body             string  `bun:"body"`
	SenderName       string  `bun:"sender_name"`
	AttachmentName   *string `bun:"attachment_name"`
}

// Execute 在会话级咨询锁内取不早于触发回复、晚于客户已读位置与已通知位置的真人回复并合并发送一封邮件；发信成功后推进已通知位置并写入成员可见事件。
// 部署未配置发信、非网站渠道、联系人没有邮箱或没有待通知回复时直接结束；发信失败时回滚并按任务重试。
func (w *Worker) Execute(ctx context.Context, input NotifyInput) error {
	if w.sender == nil {
		return nil
	}
	return realtime.RunInTx(ctx, w.db, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 2))", "customer-email-notify:"+input.ConversationID); err != nil {
			return fmt.Errorf("lock customer email notification: %w", err)
		}
		recipient, err := loadCustomerRecipient(ctx, tx, input.OrganizationID, input.ConversationID)
		if err != nil {
			return err
		}
		if domain.ChannelType(recipient.ChannelType) != domain.ChannelTypeWebsite || recipient.Email == nil {
			return nil
		}
		customer := &servermodels.CustomerConversation{}
		if err := tx.NewSelect().Model(customer).Column("cc.customer_read_seq", "cc.customer_notified_seq").
			Where("cc.organization_id = ? AND cc.conversation_id = ?", input.OrganizationID, input.ConversationID).
			Scan(ctx); err != nil {
			return fmt.Errorf("load customer notification positions: %w", err)
		}
		var replies []pendingReply
		if err := humanRepliesQuery(tx, input.OrganizationID, input.ConversationID).
			ColumnExpr("msg.message_seq, msg.service_session_id, msg.body, oi.display_name AS sender_name, ma.name AS attachment_name").
			Join("LEFT JOIN message_attachments AS ma ON ma.organization_id = msg.organization_id AND ma.message_id = msg.id").
			Where("msg.message_seq >= ? AND msg.message_seq > ? AND msg.message_seq > ?", input.MessageSeq, customer.CustomerReadSeq, customer.CustomerNotifiedSeq).
			OrderExpr("msg.message_seq").
			Scan(ctx, &replies); err != nil {
			return fmt.Errorf("load pending customer replies: %w", err)
		}
		if len(replies) == 0 {
			return nil
		}
		message, err := w.composeMessage(ctx, tx, input, recipient, replies)
		if err != nil {
			return err
		}
		if err := w.sender.Send(ctx, message); err != nil {
			return err
		}
		return recordNotified(ctx, tx, input, *recipient.Email, replies[len(replies)-1])
	})
}

// composeMessage 按客户语言生成邮件；匿名访客附带回访链接，签名身份访客提示登录企业网站继续对话。
func (w *Worker) composeMessage(ctx context.Context, tx bun.Tx, input NotifyInput, recipient customerRecipient, replies []pendingReply) (mail.Message, error) {
	organization := &servermodels.Organization{}
	if err := tx.NewSelect().Model(organization).Column("o.name", "o.access_host").
		Where("o.id = ?", input.OrganizationID).Scan(ctx); err != nil {
		return mail.Message{}, fmt.Errorf("load notification organization: %w", err)
	}
	channel, err := chatstate.LoadConversationChannel(ctx, tx, input.OrganizationID, input.ConversationID)
	if err != nil {
		return mail.Message{}, err
	}
	locale, err := translationaction.CustomerLocale(ctx, tx, input.OrganizationID, input.ConversationID, domain.Locale(channel.DefaultLocale))
	if err != nil {
		return mail.Message{}, err
	}
	content := notificationContent{Locale: locale, Organization: organization.Name, Recipient: *recipient.Email}
	for _, reply := range replies {
		content.Replies = append(content.Replies, notificationReply{SenderName: reply.SenderName, Body: reply.Body, AttachmentName: reply.AttachmentName})
	}
	// 匿名访客凭回访令牌回到原会话，签名身份访客由企业网站恢复登录身份。
	if strings.HasPrefix(recipient.ExternalID, websiteVisitorExternalIDPrefix) {
		token, err := issueResumeToken(ctx, tx, input, recipient.ChannelIdentityID)
		if err != nil {
			return mail.Message{}, err
		}
		content.ResumeURL = w.scheme + "://" + organization.AccessHost + "/chat/" + channel.ID + "?resume=" + token
	}
	return renderNotification(content)
}

// recordNotified 在会话锁内推进已通知位置，并写入只对成员可见的邮件通知事件。
func recordNotified(ctx context.Context, tx bun.Tx, input NotifyInput, address string, last pendingReply) error {
	conversation, err := chatstate.LockCustomerConversation(ctx, tx, input.OrganizationID, input.ConversationID)
	if err != nil {
		return err
	}
	if _, err := tx.NewUpdate().Model((*servermodels.CustomerConversation)(nil)).
		Set("customer_notified_seq = GREATEST(customer_notified_seq, ?)", last.MessageSeq).
		Set("updated_at = now()").
		Where("organization_id = ? AND conversation_id = ?", input.OrganizationID, input.ConversationID).
		Exec(ctx); err != nil {
		return fmt.Errorf("advance customer notified position: %w", err)
	}
	payload, err := json.Marshal(domain.ServiceSessionEmailEvent{ServiceSessionID: last.ServiceSessionID, Email: address})
	if err != nil {
		return fmt.Errorf("encode email notified event: %w", err)
	}
	eventType := string(domain.ConversationSystemEventServiceSessionEmailNotified)
	if _, _, err := chatstate.AppendMessage(ctx, tx, conversation, &servermodels.Message{
		ID: uuid.NewV7().String(), OrganizationID: input.OrganizationID, ConversationID: input.ConversationID,
		ServiceSessionID: &last.ServiceSessionID, Type: string(domain.MessageTypeSystem), Visibility: string(domain.MessageVisibilityInternalOnly),
		SystemEventType: &eventType, SystemEventPayload: payload,
	}); err != nil {
		return fmt.Errorf("append email notified event: %w", err)
	}
	slog.Info("客服回复已通过邮件通知客户",
		"organization_id", input.OrganizationID, "conversation_id", input.ConversationID, "last_message_seq", last.MessageSeq)
	return nil
}

// issueResumeToken 签发绑定渠道身份与客户会话的回访令牌，只保存令牌摘要。
func issueResumeToken(ctx context.Context, tx bun.Tx, input NotifyInput, channelIdentityID string) (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate resume token: %w", err)
	}
	token := hex.EncodeToString(value)
	if _, err := tx.NewInsert().Model(&servermodels.ConversationResumeToken{
		ID: uuid.NewV7().String(), OrganizationID: input.OrganizationID, ConversationID: input.ConversationID,
		ContactChannelIdentityID: channelIdentityID, TokenHash: ResumeTokenHash(token), ExpiresAt: time.Now().Add(ResumeTokenTTL),
	}).Exec(ctx); err != nil {
		return "", fmt.Errorf("save resume token: %w", err)
	}
	return token, nil
}

// ResumeTokenHash 返回回访令牌原文的 SHA-256 十六进制摘要。
func ResumeTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
