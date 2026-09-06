//go:build server

package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestCustomerReplies 验证客服引用、客户身份、公开摘要、幂等和删除后的展示。
func TestCustomerReplies(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	original, err := f.visitorMessage(ctx, "第一问 <script>alert(1)</script>\n第二行")
	if err != nil {
		t.Fatal(err)
	}
	name := "渠道客户"
	if _, err := f.db.NewUpdate().Table("contact_channel_identities").Set("display_name = ?", name).Where("channel_id = ?", f.channelID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	send := conversationaction.NewSendCustomerTextMessageAction(f.db)
	input := conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "回答第一问", ReplyToMessageID: original.Message.ID}
	reply, err := send.Execute(ctx, f.owner, input)
	if err != nil || reply.ReplyTo == nil || reply.ReplyTo.Body != original.Message.Body || reply.ReplyTo.Sender.Kind != domain.ChatSubjectKindContact || reply.ReplyTo.Sender.DisplayName == nil || *reply.ReplyTo.Sender.DisplayName != name {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
	replay, err := send.Execute(ctx, f.owner, input)
	if err != nil || replay.ID != reply.ID || replay.ReplyTo == nil || *replay.ReplyTo.Sender.DisplayName != name {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	for _, change := range []conversationaction.CustomerTextMessageInput{
		{ConversationID: f.conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body},
		{ConversationID: f.conversationID, ClientMessageID: input.ClientMessageID, Body: "改过的回答", ReplyToMessageID: original.Message.ID},
		{ConversationID: f.conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: reply.ID},
	} {
		_, err := send.Execute(ctx, f.owner, change)
		var conflict *conversationaction.ConflictError
		if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonIdempotencyMismatch {
			t.Fatalf("changed retry=%v", err)
		}
	}
	history, err := conversationaction.NewListConversationMessagesQuery(f.db).Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID})
	if err != nil {
		t.Fatal(err)
	}
	reference := history.Messages[len(history.Messages)-1].ReplyTo
	if reference == nil || reference.Body != original.Message.Body || *reference.Sender.DisplayName != name {
		t.Fatalf("history reference=%+v", reference)
	}
	own, err := send.Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "补充说明", ReplyToMessageID: reply.ID})
	if err != nil || own.ReplyTo == nil || own.ReplyTo.Sender.SourceID != f.owner.OrganizationIdentity.ID {
		t.Fatalf("own reference=%+v err=%v", own.ReplyTo, err)
	}
	visitor := appservice.NewWebsiteVisitorDirectBackend(f.db, nil)
	page, err := visitor.ListMessages(ctx, appservice.WebsiteVisitorMeta{}, f.channelID, "web-session:0123456789abcdef0123456789abcdef", f.conversationID, appservice.WebsiteVisitorMessageHistoryInput{})
	if err != nil {
		t.Fatal(err)
	}
	if r := page.Messages[len(page.Messages)-2].ReplyTo; r == nil || r.Author != "visitor" || r.Body != original.Message.Body {
		t.Fatalf("visitor reference=%+v", r)
	}
	if r := page.Messages[len(page.Messages)-1].ReplyTo; r == nil || r.Author != "agent" || r.Body != input.Body {
		t.Fatalf("agent reference=%+v", r)
	}
	if _, err := f.db.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id = ?", original.Message.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	replay, err = send.Execute(ctx, f.owner, input)
	if err != nil || replay.ID != reply.ID || replay.ReplyTo == nil || !replay.ReplyTo.Deleted || replay.ReplyTo.Body != "" || replay.ReplyTo.Sender != nil {
		t.Fatalf("deleted replay=%+v err=%v", replay, err)
	}
	history, err = conversationaction.NewListConversationMessagesQuery(f.db).Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID})
	if err != nil {
		t.Fatal(err)
	}
	if r := history.Messages[len(history.Messages)-2].ReplyTo; r == nil || !r.Deleted || r.Body != "" || r.Sender != nil {
		t.Fatalf("deleted member reference=%+v", r)
	}
	page, err = visitor.ListMessages(ctx, appservice.WebsiteVisitorMeta{}, f.channelID, "web-session:0123456789abcdef0123456789abcdef", f.conversationID, appservice.WebsiteVisitorMessageHistoryInput{})
	if err != nil {
		t.Fatal(err)
	}
	if r := page.Messages[len(page.Messages)-2].ReplyTo; r == nil || !r.Deleted || r.Body != "" || r.Author != "" {
		t.Fatalf("deleted visitor reference=%+v", r)
	}
}

// TestCustomerReplyBoundaries 验证无效引用不领取会话，关闭和他人负责时不能发送。
func TestCustomerReplyBoundaries(t *testing.T) {
	f := newCustomerReadFixture(t)
	foreign := newCustomerReadFixture(t)
	ctx := context.Background()
	original, err := f.visitorMessage(ctx, "原文")
	if err != nil {
		t.Fatal(err)
	}
	foreignMessage, err := foreign.visitorMessage(ctx, "其他企业")
	if err != nil {
		t.Fatal(err)
	}
	other := f.send(t, f.owner, "同企业其他会话", false)
	system := &servermodels.Message{OrganizationID: f.owner.Organization.ID, ConversationID: f.conversationID, Type: "system", Body: "系统事件", OriginatedAt: time.Now().UTC()}
	if _, err := f.db.NewInsert().Model(system).Column("organization_id", "conversation_id", "type", "body", "originated_at").Returning("id").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id = ?", original.Message.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	send := conversationaction.NewSendCustomerTextMessageAction(f.db)
	before, err := f.db.NewSelect().Model((*servermodels.Message)(nil)).Where("msg.conversation_id = ?", f.conversationID).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{original.Message.ID, foreignMessage.Message.ID, other.ID, system.ID, uuid.NewV7().String()} {
		_, err := send.Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "无效回复", ReplyToMessageID: target})
		var conflict *conversationaction.ConflictError
		if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonReplyTargetInvalid {
			t.Fatalf("target=%s err=%v", target, err)
		}
	}
	after, err := f.db.NewSelect().Model((*servermodels.Message)(nil)).Where("msg.conversation_id = ?", f.conversationID).Count(ctx)
	if err != nil || after != before {
		t.Fatalf("invalid writes: %d -> %d, err=%v", before, after, err)
	}
	session := &servermodels.ServiceSession{}
	if err := f.db.NewSelect().Model(session).Where("ss.conversation_id = ? AND ss.status = ?", f.conversationID, domain.ServiceSessionStatusOpen).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if session.AssigneeIdentityID != nil {
		t.Fatalf("invalid reference claimed queue: %+v", session)
	}
	valid, err := f.visitorMessage(ctx, "有效问题")
	if err != nil {
		t.Fatal(err)
	}
	input := conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "有效回复", ReplyToMessageID: valid.Message.ID}
	if _, err := send.Execute(ctx, foreign.owner, input); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("foreign sender=%v", err)
	}
	if _, err := send.Execute(ctx, f.owner, input); err != nil {
		t.Fatal(err)
	}
	input.ClientMessageID = uuid.NewV7().String()
	_, err = send.Execute(ctx, f.member, input)
	var conflict *conversationaction.ConflictError
	if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonServiceSessionOwned {
		t.Fatalf("other assignee=%v", err)
	}
	if _, err := conversationaction.NewCloseServiceSessionAction(f.db, agentrunaction.NewExecuteAction(f.db, nil, nil, nil)).Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatal(err)
	}
	_, err = send.Execute(ctx, f.owner, input)
	if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonServiceSessionNotReplyable {
		t.Fatalf("closed reply=%v", err)
	}
	_, err = conversationaction.NewListWebsiteMessagesQuery(f.db).Execute(ctx, conversationaction.MessageHistoryInput{ChannelID: f.channelID, ExternalID: "web-session:ffffffffffffffffffffffffffffffff", ConversationID: f.conversationID})
	if !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("foreign visitor=%v", err)
	}
}

// TestCustomerReplyEarlierSession 验证跨客服周期引用和窗口外定位不会改变个人阅读状态。
func TestCustomerReplyEarlierSession(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	original, err := f.visitorMessage(ctx, "上一处理周期的问题")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewClaimServiceSessionAction(f.db, agentrunaction.NewExecuteAction(f.db, nil, nil, nil)).Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewCloseServiceSessionAction(f.db, agentrunaction.NewExecuteAction(f.db, nil, nil, nil)).Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatal(err)
	}
	for i := range 60 {
		result, err := f.visitorMessage(ctx, fmt.Sprintf("后续问题 %d", i))
		if err != nil || (i == 0 && !result.OpenedNewServiceSession) {
			t.Fatalf("new cycle=%+v err=%v", result, err)
		}
	}
	reply, err := conversationaction.NewSendCustomerTextMessageAction(f.db).Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "针对早期问题", ReplyToMessageID: original.Message.ID})
	if err != nil || reply.ReplyTo == nil || reply.ReplyTo.ID != original.Message.ID {
		t.Fatalf("earlier reply=%+v err=%v", reply, err)
	}
	query := conversationaction.NewListConversationMessagesQuery(f.db)
	latest, err := query.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID})
	if err != nil || !latest.HasEarlier || latest.Messages[0].ID == original.Message.ID {
		t.Fatalf("latest=%+v err=%v", latest, err)
	}
	window, err := query.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID, AroundMessageID: original.Message.ID})
	if err != nil || !window.HasLater || len(window.Messages) != 27 || window.Messages[1].ID != original.Message.ID {
		t.Fatalf("window=%+v err=%v", window, err)
	}
	count, err := f.db.NewSelect().Model((*servermodels.ConversationUserState)(nil)).Where("cus.conversation_id = ? AND cus.user_id = ?", f.conversationID, f.member.User.ID).Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("location changed read: count=%d err=%v", count, err)
	}
}
