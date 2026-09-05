//go:build server

package server

import (
	"context"
	"errors"
	"testing"
	"uuid"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestConversationUnreadMark 验证个人标记、静音和两个阅读水位相互独立。
func TestConversationUnreadMark(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	mark := conversationaction.NewUpdateConversationUnreadMarkAction(f.db)
	read := conversationaction.NewMarkConversationReadAction(f.db)
	review := conversationaction.NewMarkConversationMentionReviewedAction(f.db)
	mute := conversationaction.NewUpdateConversationNotificationSettingsAction(f.db)
	inbox := inboxaction.NewLoadInboxQuery(f.db)
	first := f.send(t, f.owner, "提及", false, f.subjectID)
	if _, err := review.Execute(ctx, f.member, f.groupID, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := read.Execute(ctx, f.member, f.groupID, first.ID, false); err != nil {
		t.Fatal(err)
	}
	before := f.state(t)
	for range 2 {
		if err := mark.Execute(ctx, f.member, f.groupID, true); err != nil {
			t.Fatal(err)
		}
	}
	state := f.state(t)
	if !state.MarkedUnread || *state.LastReadMessageID != *before.LastReadMessageID || !state.LastReadAt.Equal(*before.LastReadAt) || *state.LastReviewedMentionMessageID != *before.LastReviewedMentionMessageID {
		t.Fatalf("mark changed read facts: %#v", state)
	}
	rows, counts, err := inbox.Execute(ctx, f.member, inboxaction.LoadInput{Scope: domain.InboxScopeInternal})
	if err != nil || len(rows) != 1 || !rows[0].MarkedUnread || counts.Unread != 0 || counts.Attention != 1 {
		t.Fatalf("marked inbox = %#v %#v %v", rows, counts, err)
	}
	f.send(t, f.owner, "普通消息", false)
	_, counts, err = inbox.Execute(ctx, f.member, inboxaction.LoadInput{})
	if err != nil || counts.Unread != 1 || counts.Attention != 1 {
		t.Fatalf("mark double counted: %#v %v", counts, err)
	}
	if _, err = mute.Execute(ctx, f.member, f.groupID, true); err != nil {
		t.Fatal(err)
	}
	_, counts, err = inbox.Execute(ctx, f.member, inboxaction.LoadInput{})
	if err != nil || counts.Attention != 0 {
		t.Fatalf("muted mark: %#v %v", counts, err)
	}
	last := f.send(t, f.owner, "再次提及", true, f.subjectID)
	_, counts, err = inbox.Execute(ctx, f.member, inboxaction.LoadInput{})
	if err != nil || counts.Unread != 2 || counts.Attention != 1 {
		t.Fatalf("muted mention: %#v %v", counts, err)
	}
	if _, err = read.Execute(ctx, f.member, f.groupID, last.ID, false); err != nil {
		t.Fatal(err)
	}
	if !f.state(t).MarkedUnread {
		t.Fatal("automatic read cleared manual mark")
	}
	if _, err = mute.Execute(ctx, f.member, f.groupID, false); err != nil {
		t.Fatal(err)
	}
	_, counts, err = inbox.Execute(ctx, f.member, inboxaction.LoadInput{})
	if err != nil || counts.Unread != 0 || counts.Attention != 1 {
		t.Fatalf("manual reminder lost: %#v %v", counts, err)
	}
	if err = mark.Execute(ctx, f.member, f.groupID, false); err != nil {
		t.Fatal(err)
	}
	state = f.state(t)
	if state.MarkedUnread || *state.LastReadMessageID != last.ID || *state.LastReviewedMentionMessageID != first.ID {
		t.Fatalf("clear changed read facts: %#v", state)
	}
	_, counts, err = inbox.Execute(ctx, f.member, inboxaction.LoadInput{})
	if err != nil || counts.Unread != 0 || counts.Attention != 0 {
		t.Fatalf("read inbox: %#v %v", counts, err)
	}
	if err = mark.Execute(ctx, f.member, f.groupID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = read.Execute(ctx, f.member, f.groupID, uuid.NewV7().String(), true); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("invalid read target = %v", err)
	}
	if !f.state(t).MarkedUnread {
		t.Fatal("failed read cleared unread mark")
	}
	if _, err = read.Execute(ctx, f.member, f.groupID, last.ID, true); err != nil {
		t.Fatal(err)
	}
	if f.state(t).MarkedUnread {
		t.Fatal("explicit read did not clear unread mark at unchanged watermark")
	}

	other := newNavigationFixture(t)
	if err = mark.Execute(ctx, other.member, f.groupID, true); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("cross organization = %v", err)
	}
	if _, err = f.db.NewUpdate().Table("conversation_participants").Set("left_at = now()").Where("organization_id = ? AND conversation_id = ? AND subject_id = ?", f.member.Organization.ID, f.groupID, f.subjectID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if err = mark.Execute(ctx, f.member, f.groupID, true); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("former member = %v", err)
	}
}

// TestInboxUnreadUsesCanonicalDirect 验证单聊列表与总数共用身份对来源，避免隐藏会话留下红点。
func TestInboxUnreadUsesCanonicalDirect(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	sent, err := conversationaction.NewSendFirstDirectTextMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.FirstDirectTextMessageInput{TargetIdentityID: f.member.OrganizationIdentity.ID, ClientMessageID: uuid.NewV7().String(), Body: "单聊消息"})
	if err != nil {
		t.Fatal(err)
	}
	inbox := inboxaction.NewLoadInboxQuery(f.db)
	mark := conversationaction.NewUpdateConversationUnreadMarkAction(f.db)
	read := conversationaction.NewMarkConversationReadAction(f.db)
	if _, err = read.Execute(ctx, f.member, sent.Conversation.ID, sent.Message.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = mark.Execute(ctx, f.member, sent.Conversation.ID, true); err != nil {
		t.Fatal(err)
	}
	_, counts, err := inbox.Execute(ctx, f.member, inboxaction.LoadInput{})
	if err != nil || counts.Unread != 0 || counts.Attention != 1 {
		t.Fatalf("direct mark: %#v %v", counts, err)
	}
	if _, err = conversationaction.NewUpdateConversationNotificationSettingsAction(f.db).Execute(ctx, f.member, sent.Conversation.ID, true); err != nil {
		t.Fatal(err)
	}
	_, counts, err = inbox.Execute(ctx, f.member, inboxaction.LoadInput{})
	if err != nil || counts.Attention != 0 {
		t.Fatalf("muted direct mark: %#v %v", counts, err)
	}
	// 模拟迁移前没有规范化身份对、但仍有参与者和消息的会话。
	if _, err = f.db.NewDelete().Table("direct_conversations").Where("organization_id = ? AND conversation_id = ?", f.owner.Organization.ID, sent.Conversation.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.NewDelete().Table("conversation_user_states").Where("organization_id = ? AND conversation_id = ?", f.owner.Organization.ID, sent.Conversation.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	rows, counts, err := inbox.Execute(ctx, f.member, inboxaction.LoadInput{})
	if err != nil || len(rows) != 1 || rows[0].ID != f.groupID || counts.Unread != 0 || counts.Attention != 0 {
		t.Fatalf("hidden direct leaked unread: %#v %#v %v", rows, counts, err)
	}
}

// TestEmptyConversationUnreadMarksIgnoreListLimit 验证空群标记、跨筛选汇总和列表条数上限。
func TestEmptyConversationUnreadMarksIgnoreListLimit(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	mark := conversationaction.NewUpdateConversationUnreadMarkAction(f.db)
	create := conversationaction.NewCreateGroupConversationAction(f.db)
	if err := mark.Execute(ctx, f.member, f.groupID, true); err != nil {
		t.Fatal(err)
	}
	state := f.state(t)
	if !state.MarkedUnread || state.LastReadMessageID != nil || state.LastReadAt != nil || state.LastReviewedMentionMessageID != nil {
		t.Fatalf("empty mark created watermarks: %#v", state)
	}
	if err := mark.Execute(ctx, f.member, f.groupID, false); err != nil {
		t.Fatal(err)
	}
	for range 51 {
		group, err := create.Execute(ctx, f.owner, conversationaction.GroupConversationInput{Title: "未读标记测试", MemberIdentityIDs: []string{f.member.OrganizationIdentity.ID}})
		if err != nil {
			t.Fatal(err)
		}
		if err = mark.Execute(ctx, f.member, group.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	inbox := inboxaction.NewLoadInboxQuery(f.db)
	rows, counts, err := inbox.Execute(ctx, f.member, inboxaction.LoadInput{Scope: domain.InboxScopeInternal})
	if err != nil || len(rows) != 50 || counts.Unread != 0 || counts.Attention != 51 {
		t.Fatalf("limited list changed total: %d %#v %v", len(rows), counts, err)
	}
	rows, counts, err = inbox.Execute(ctx, f.member, inboxaction.LoadInput{Scope: domain.InboxScopeCustomer})
	if err != nil || len(rows) != 0 || counts.Attention != 51 {
		t.Fatalf("customer scope changed total: %d %#v %v", len(rows), counts, err)
	}
}
