//go:build server

package integrationtest

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/actions/customernotify"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/mail"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// handoffEmailRequest 是渠道语言 zh-CN 下转人工话术追加的留邮箱请求。
const handoffEmailRequest = "如需离开，可以在这里留下邮箱，客服回复后我们会发邮件通知您。"

// recordingMailSender 记录发出的邮件，fail 非空时返回该错误。
type recordingMailSender struct {
	mu       sync.Mutex
	messages []mail.Message
	fail     error
}

// Send 记录邮件或返回预设错误。
func (s *recordingMailSender) Send(_ context.Context, message mail.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return s.fail
	}
	s.messages = append(s.messages, message)
	return nil
}

// sent 返回已记录的邮件。
func (s *recordingMailSender) sent() []mail.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]mail.Message(nil), s.messages...)
}

// testCustomerEmailNotification 验证转人工后收集访客邮箱、访客未读时合并发送邮件通知，以及凭回访令牌回到原会话。
func testCustomerEmailNotification(t *testing.T, db *bun.DB, identity *servermodels.Identity, providerID, modelID string) {
	ctx := context.Background()
	tasks := newTestTasks(db)
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	f := handoffFixture{db: db, identity: identity, tasks: tasks, providerID: providerID, modelID: modelID}
	disableAutoAssignment(t, db, identity.Organization.ID)
	sender := &recordingMailSender{}
	agent := f.newAgent(t, "邮件通知客服")
	channelID := f.newChannel(t, agent.IdentityID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	input := visitorInput(channelID, "")
	receive := func(body string) conversationaction.ReceiveWebsiteCustomerMessageResult {
		t.Helper()
		input.ClientMessageID, input.Body = uuid.NewV7().String(), body
		result, err := conversationaction.NewReceiveWebsiteCustomerMessageAction(db, agentrunaction.NewScheduler(tasks), newTestTasks(db), sender).Execute(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		input.ConversationID = &result.Conversation.ID
		return result
	}

	// 转人工进入公共队列时，话术末尾请访客留下邮箱。
	first := receive("我要退款")
	conversationID := first.Conversation.ID
	run := f.queuedRun(t, conversationID)
	if err := agentrunaction.NewExecuteAction(db, tasks, handoffRuntime("客户要求退款", nil), testAttachmentReader(db), nil, sender).Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	if notice := handoffNotice(t, db, "agent:"+run.ID); notice != handoffQueuedNotice+"\n\n"+handoffEmailRequest {
		t.Fatalf("handoff notice = %q", notice)
	}

	// 转人工后访客消息中的唯一邮箱写入联系人，并向访客展示留邮箱事件；已有邮箱后不再收集。
	receive("我的邮箱是 Visitor@Example.com。")
	receive("备用邮箱 other@example.com")
	var emails []string
	if err := db.NewSelect().TableExpr("contact_methods AS cm").ColumnExpr("cm.normalized_value").
		Join("JOIN contact_channel_identities AS cci ON cci.contact_id = cm.contact_id").
		Where("cci.external_id = ? AND cm.type = ? AND cm.is_primary", input.ExternalID, domain.ContactMethodTypeEmail).
		Scan(ctx, &emails); err != nil || len(emails) != 1 || emails[0] != "visitor@example.com" {
		t.Fatalf("contact emails = %v, error = %v", emails, err)
	}
	history, err := conversationaction.NewListWebsiteMessagesQuery(db).Execute(ctx, conversationaction.MessageHistoryInput{ChannelID: channelID, ExternalID: input.ExternalID, ConversationID: conversationID})
	if err != nil {
		t.Fatal(err)
	}
	collected := 0
	for _, message := range history.Messages {
		if message.Event != nil && message.Event.Type == conversationaction.VisitorEventEmailCollected {
			collected++
			if message.Event.Email != "visitor@example.com" {
				t.Fatalf("email collected event = %+v", message.Event)
			}
		}
	}
	if collected != 1 {
		t.Fatalf("email collected events = %d", collected)
	}

	// 真人连续回复各登记一次延迟检查；访客读到第一条后，第一条的检查只通知其后的回复。
	send := conversationaction.NewSendCustomerTextMessageAction(db, newTestTasks(db))
	reply := func(body string) conversationaction.ConversationMessage {
		t.Helper()
		message, err := send.Execute(ctx, identity, conversationaction.CustomerTextMessageInput{ConversationID: conversationID, ClientMessageID: uuid.NewV7().String(), Body: body})
		if err != nil {
			t.Fatal(err)
		}
		return message
	}
	firstReply := reply("已收到您的退款申请")
	secondReply := reply("退款将在三个工作日内到账")
	notifications := notificationInputs(t, db, conversationID)
	if len(notifications) != 2 || notifications[0].MessageSeq != firstReply.MessageSeq || notifications[1].MessageSeq != secondReply.MessageSeq {
		t.Fatalf("notification tasks = %+v", notifications)
	}
	if err := conversationaction.NewMarkWebsiteConversationReadAction(db).Execute(ctx, channelID, input.ExternalID, conversationID, firstReply.MessageSeq); err != nil {
		t.Fatal(err)
	}
	worker := customernotify.NewWorker(db, sender, "https")
	for _, notification := range notifications {
		if err := worker.Execute(ctx, notification); err != nil {
			t.Fatal(err)
		}
	}
	sent := sender.sent()
	if len(sent) != 1 || sent[0].To != "visitor@example.com" || !strings.Contains(sent[0].Text, "退款将在三个工作日内到账") || strings.Contains(sent[0].Text, "已收到您的退款申请") {
		t.Fatalf("sent emails = %+v", sent)
	}
	notified := customerPositions(t, db, conversationID)
	if notified.CustomerReadSeq != firstReply.MessageSeq || notified.CustomerNotifiedSeq != secondReply.MessageSeq {
		t.Fatalf("customer positions = %+v", notified)
	}
	if count := emailNotifiedEvents(t, db, conversationID); count != 1 {
		t.Fatalf("email notified events = %d", count)
	}

	// 回访令牌可重复换取原访客令牌并打开原会话，错误令牌与其他渠道不可用。
	resumeURL := resumeLink(t, sent[0].Text)
	token := resumeURL.Query().Get("resume")
	if resumeURL.Path != "/chat/"+channelID || token == "" {
		t.Fatalf("resume url = %s", resumeURL)
	}
	resume := conversationaction.NewResumeWebsiteVisitorQuery(db)
	for range 2 {
		resumed, err := resume.Execute(ctx, channelID, token)
		if err != nil || "web-session:"+resumed.VisitorToken != input.ExternalID || resumed.ConversationID != conversationID {
			t.Fatalf("resumed = %+v, error = %v", resumed, err)
		}
	}
	if _, err := resume.Execute(ctx, channelID, strings.Repeat("0", 64)); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("invalid token error = %v", err)
	}
	otherChannelID := f.newChannel(t, agent.IdentityID, channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue})
	if _, err := resume.Execute(ctx, otherChannelID, token); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("other channel token error = %v", err)
	}

	// 发信失败时回滚并保留水位，恢复后重试发送；未配置发信时任务直接结束。
	thirdReply := reply("请确认收款账户")
	sender.fail = errors.New("smtp unavailable")
	third := customernotify.NotifyInput{OrganizationID: identity.Organization.ID, ConversationID: conversationID, MessageSeq: thirdReply.MessageSeq}
	if err := worker.Execute(ctx, third); err == nil {
		t.Fatal("notification succeeded while smtp is unavailable")
	}
	if positions := customerPositions(t, db, conversationID); positions.CustomerNotifiedSeq != secondReply.MessageSeq || emailNotifiedEvents(t, db, conversationID) != 1 {
		t.Fatalf("positions after failure = %+v", positions)
	}
	if err := customernotify.NewWorker(db, nil, "https").Execute(ctx, third); err != nil {
		t.Fatal(err)
	}
	sender.fail = nil
	if err := worker.Execute(ctx, third); err != nil {
		t.Fatal(err)
	}
	if sent := sender.sent(); len(sent) != 2 || !strings.Contains(sent[1].Text, "请确认收款账户") || customerPositions(t, db, conversationID).CustomerNotifiedSeq != thirdReply.MessageSeq {
		t.Fatalf("sent after retry = %+v", sent)
	}
}

// notificationInputs 读取会话已登记的邮件通知检查任务。
func notificationInputs(t *testing.T, db *bun.DB, conversationID string) []customernotify.NotifyInput {
	t.Helper()
	var runs []servermodels.TaskRun
	if err := db.NewSelect().Model(&runs).
		Where("tr.action_name = ? AND tr.payload->>'conversationId' = ?", customernotify.NotifyActionName, conversationID).
		OrderExpr("tr.created_at").Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	inputs := make([]customernotify.NotifyInput, 0, len(runs))
	for _, run := range runs {
		input := customernotify.NotifyInput{}
		if err := json.Unmarshal(run.Payload, &input); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, input)
	}
	return inputs
}

// customerPositions 读取客户已读位置与已通知位置。
func customerPositions(t *testing.T, db *bun.DB, conversationID string) servermodels.CustomerConversation {
	t.Helper()
	customer := servermodels.CustomerConversation{}
	if err := db.NewSelect().Model(&customer).Where("cc.conversation_id = ?", conversationID).Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	return customer
}

// emailNotifiedEvents 统计会话中只对成员可见的邮件通知事件。
func emailNotifiedEvents(t *testing.T, db *bun.DB, conversationID string) int {
	t.Helper()
	count, err := db.NewSelect().Model((*servermodels.Message)(nil)).
		Where("msg.conversation_id = ? AND msg.system_event_type = ? AND msg.visibility = ?", conversationID,
			domain.ConversationSystemEventServiceSessionEmailNotified, domain.MessageVisibilityInternalOnly).
		Count(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return count
}

// resumeLink 从纯文本邮件中取出「继续对话」链接。
func resumeLink(t *testing.T, text string) *url.URL {
	t.Helper()
	for _, field := range strings.Fields(text) {
		if strings.HasPrefix(field, "https://") {
			parsed, err := url.Parse(field)
			if err != nil {
				t.Fatal(err)
			}
			return parsed
		}
	}
	t.Fatalf("resume link missing in %q", text)
	return nil
}
