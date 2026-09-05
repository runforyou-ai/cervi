//go:build server

package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
	"uuid"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestDirectMessageReplies 验证单聊双方引用、幂等重放及删除后的引用摘要。
func TestDirectMessageReplies(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	first, err := conversationaction.NewSendFirstDirectTextMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.FirstDirectTextMessageInput{
		TargetIdentityID: f.member.OrganizationIdentity.ID, ClientMessageID: uuid.NewV7().String(), Body: "原文",
	})
	if err != nil {
		t.Fatal(err)
	}
	send := conversationaction.NewSendDirectTextMessageAction(f.db, nil)
	input := conversationaction.DirectTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "回复", ReplyToMessageID: first.Message.ID}
	reply, err := send.Execute(ctx, f.member, input)
	if err != nil || reply.ReplyTo == nil || reply.ReplyTo.Body != "原文" || reply.ReplyTo.Sender.SourceID != f.owner.OrganizationIdentity.ID {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
	replay, err := send.Execute(ctx, f.member, input)
	if err != nil || replay.ID != reply.ID || replay.ReplyTo == nil || replay.ReplyTo.ID != first.Message.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	for _, target := range []string{"", reply.ID} {
		changed := input
		changed.ReplyToMessageID = target
		_, err := send.Execute(ctx, f.member, changed)
		var conflict *conversationaction.ConflictError
		if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonIdempotencyMismatch {
			t.Fatalf("changed target accepted: %v", err)
		}
	}
	input.ClientMessageID = uuid.NewV7().String()
	input.ReplyToMessageID = reply.ID
	ownReply, err := send.Execute(ctx, f.member, input)
	if err != nil || ownReply.ReplyTo == nil || ownReply.ReplyTo.ID != reply.ID || ownReply.ReplyTo.Sender.SourceID != f.member.OrganizationIdentity.ID {
		t.Fatalf("own reply=%+v err=%v", ownReply, err)
	}
	if _, err := f.db.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id = ?", reply.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	replay, err = send.Execute(ctx, f.member, input)
	if err != nil || replay.ID != ownReply.ID || replay.ReplyTo == nil || !replay.ReplyTo.Deleted || replay.ReplyTo.Body != "" || replay.ReplyTo.Sender != nil {
		t.Fatalf("deleted replay=%+v err=%v", replay, err)
	}
	history, err := conversationaction.NewListConversationMessagesQuery(f.db).Execute(ctx, f.owner, conversationaction.ConversationMessageHistoryInput{ConversationID: first.Conversation.ID})
	if err != nil {
		t.Fatal(err)
	}
	last := history.Messages[len(history.Messages)-1]
	if last.ID != ownReply.ID || last.ReplyTo == nil || !last.ReplyTo.Deleted || last.ReplyTo.Body != "" || last.ReplyTo.Sender != nil {
		t.Fatalf("history reference=%+v", last.ReplyTo)
	}
}

// TestDirectReplyBoundaries 验证引用目标的会话、企业、消息类型和删除边界。
func TestDirectReplyBoundaries(t *testing.T) {
	f := newNavigationFixture(t)
	foreign := newNavigationFixture(t)
	ctx := context.Background()
	first, err := conversationaction.NewSendFirstDirectTextMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.FirstDirectTextMessageInput{
		TargetIdentityID: f.member.OrganizationIdentity.ID, ClientMessageID: uuid.NewV7().String(), Body: "原文",
	})
	if err != nil {
		t.Fatal(err)
	}
	otherConversation := f.send(t, f.owner, "同企业群消息", false)
	otherOrganization := foreign.send(t, foreign.owner, "其他企业消息", false)
	system := &servermodels.Message{OrganizationID: f.owner.Organization.ID, ConversationID: first.Conversation.ID, Type: "system", Body: "系统事件", OriginatedAt: time.Now().UTC()}
	if _, err := f.db.NewInsert().Model(system).Column("organization_id", "conversation_id", "type", "body", "originated_at").Returning("id").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id = ?", first.Message.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	send := conversationaction.NewSendDirectTextMessageAction(f.db, nil)
	before, err := f.db.NewSelect().Model((*servermodels.Message)(nil)).Where("msg.conversation_id = ?", first.Conversation.ID).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{first.Message.ID, otherConversation.ID, otherOrganization.ID, system.ID, uuid.NewV7().String()} {
		_, err := send.Execute(ctx, f.member, conversationaction.DirectTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "不允许", ReplyToMessageID: target})
		var conflict *conversationaction.ConflictError
		if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonReplyTargetInvalid {
			t.Fatalf("target=%s err=%v", target, err)
		}
	}
	_, err = send.Execute(ctx, foreign.owner, conversationaction.DirectTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "越权", ReplyToMessageID: first.Message.ID})
	if !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("foreign sender error=%v", err)
	}
	after, err := f.db.NewSelect().Model((*servermodels.Message)(nil)).Where("msg.conversation_id = ?", first.Conversation.ID).Count(ctx)
	if err != nil || after != before {
		t.Fatalf("failed send persisted: before=%d after=%d err=%v", before, after, err)
	}
}

// TestDirectReplyHistoryWindow 验证单聊按原有顺序定位窗口外的引用目标且不推进已读。
func TestDirectReplyHistoryWindow(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	first, err := conversationaction.NewSendFirstDirectTextMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.FirstDirectTextMessageInput{
		TargetIdentityID: f.member.OrganizationIdentity.ID, ClientMessageID: uuid.NewV7().String(), Body: "较早的原文",
	})
	if err != nil {
		t.Fatal(err)
	}
	send := conversationaction.NewSendDirectTextMessageAction(f.db, nil)
	for i := range 60 {
		if _, err := send.Execute(ctx, f.owner, conversationaction.DirectTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: fmt.Sprintf("后续消息 %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	history := conversationaction.NewListConversationMessagesQuery(f.db)
	latest, err := history.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: first.Conversation.ID})
	if err != nil || !latest.HasEarlier || latest.Messages[0].ID == first.Message.ID {
		t.Fatalf("latest=%+v err=%v", latest, err)
	}
	window, err := history.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: first.Conversation.ID, AroundMessageID: first.Message.ID})
	if err != nil || window.HasEarlier || !window.HasLater || len(window.Messages) != 26 || window.Messages[0].ID != first.Message.ID {
		t.Fatalf("window=%+v err=%v", window, err)
	}
	count, err := f.db.NewSelect().Model((*servermodels.ConversationUserState)(nil)).Where("cus.conversation_id = ? AND cus.user_id = ?", first.Conversation.ID, f.member.User.ID).Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("history created read state: count=%d err=%v", count, err)
	}
}
