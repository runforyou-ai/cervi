//go:build server

package integrationtest

import (
	"context"
	"errors"
	"testing"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/domain"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/tenant"
)

// TestCustomerInternalNotes 验证内部备注的写入资格、周期摘要、客户侧隔离和引用边界。
func TestCustomerInternalNotes(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	visitorMessage, err := f.visitorMessage(ctx, "我的订单还没发货")
	if err != nil {
		t.Fatal(err)
	}
	send := conversationaction.NewSendServiceTextMessageAction(f.db, nil)
	noteInput := conversationaction.ServiceTextMessageInput{
		ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(),
		Body: "客户上个月投诉过物流", Visibility: domain.MessageVisibilityInternal,
	}
	// 未领取周期的其他客服同样可以留内部备注。
	note, err := send.Execute(ctx, f.member, noteInput)
	if err != nil || note.Visibility != domain.MessageVisibilityInternal {
		t.Fatalf("note=%+v err=%v", note, err)
	}

	session := &servermodels.ServiceSession{}
	if err := f.db.NewSelect().Model(session).Where("ss.conversation_id = ?", f.conversationID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if session.AssigneeIdentityID != nil || session.FirstResponseAt != nil {
		t.Fatalf("note changed service session: %+v", session)
	}
	// 周期摘要仍指向客户末条消息，转交给 AI 时据此补触发。
	if session.LastMessageID != visitorMessage.Message.ID {
		t.Fatalf("service session last message = %s, want %s", session.LastMessageID, visitorMessage.Message.ID)
	}
	conversation := &servermodels.Conversation{}
	if err := f.db.NewSelect().Model(conversation).Where("cv.id = ?", f.conversationID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if conversation.LastMessageID == nil || *conversation.LastMessageID != note.ID {
		t.Fatalf("conversation summary = %+v, want note %s", conversation.LastMessageID, note.ID)
	}
	// 成员收件箱摘要取到内部备注时标明可见范围。
	inboxPage, _, err := inboxaction.NewLoadInboxQuery(f.db).Execute(ctx, f.owner, inboxaction.LoadInput{Scope: domain.InboxScopeAll, AssigneeFilter: domain.InboxAssigneeFilterUnassigned})
	if err != nil {
		t.Fatal(err)
	}
	listed := false
	for _, row := range inboxPage.Conversations {
		if row.ID != f.conversationID {
			continue
		}
		listed = true
		if row.Service == nil || row.Service.Preview == nil || *row.Service.Preview != noteInput.Body ||
			row.Service.PreviewVisibility == nil || *row.Service.PreviewVisibility != domain.MessageVisibilityInternal {
			t.Fatalf("inbox preview = %+v", row.Service)
		}
	}
	if !listed {
		t.Fatal("customer conversation missing from inbox")
	}

	history, err := conversationaction.NewListConversationMessagesQuery(f.db).Execute(ctx, f.owner, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID})
	if err != nil {
		t.Fatal(err)
	}
	last := history.Messages[len(history.Messages)-1]
	if last.ID != note.ID || last.Visibility != domain.MessageVisibilityInternal {
		t.Fatalf("member timeline last message = %+v", last)
	}

	visitor := appservice.NewWebsiteVisitorDirectBackend(f.db, nil, newTestTasks(f.db), nil, serverfilecontent.S3Config{}, nil, nil)
	externalID := "web-session:0123456789abcdef0123456789abcdef"
	page, err := visitor.ListMessages(ctx, appservice.WebsiteVisitorMeta{}, f.channelID, externalID, f.conversationID, appservice.WebsiteVisitorMessageHistoryInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range page.Messages {
		if message.ID == note.ID || message.Body == noteInput.Body {
			t.Fatalf("visitor history contains internal note: %+v", message)
		}
	}
	threads, err := visitor.ListConversations(ctx, appservice.WebsiteVisitorMeta{}, f.channelID, externalID)
	if err != nil {
		t.Fatal(err)
	}
	for _, thread := range threads.Conversations {
		if thread.Preview == noteInput.Body {
			t.Fatalf("visitor directory preview contains internal note: %+v", thread)
		}
	}

	// 访客发送回包中的会话摘要同样只取对客消息。
	replay, err := f.receive.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: f.channelID, ExternalID: externalID, ConversationID: &f.conversationID,
		ClientMessageID: uuid.NewV7().String(), Body: "还有别的办法吗",
	})
	if err != nil {
		t.Fatal(err)
	}
	if replay.Conversation.Preview == noteInput.Body {
		t.Fatalf("visitor send summary contains internal note: %+v", replay.Conversation)
	}

	t.Run("对客消息不能引用内部备注", func(t *testing.T) {
		_, err := send.Execute(ctx, f.owner, conversationaction.ServiceTextMessageInput{
			ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(),
			Body: "已为您加急", ReplyToMessageID: note.ID,
		})
		var conflict *conversationaction.ConflictError
		if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonReplyTargetInvalid {
			t.Fatalf("customer visible reply to note = %v", err)
		}
	})

	t.Run("内部备注可以引用备注和对客消息", func(t *testing.T) {
		for _, target := range []string{note.ID, visitorMessage.Message.ID} {
			saved, err := send.Execute(ctx, f.owner, conversationaction.ServiceTextMessageInput{
				ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(),
				Body: "我来跟进", ReplyToMessageID: target, Visibility: domain.MessageVisibilityInternal,
			})
			if err != nil || saved.ReplyTo == nil || saved.ReplyTo.ID != target {
				t.Fatalf("note reply to %s = %+v err=%v", target, saved.ReplyTo, err)
			}
		}
	})

	t.Run("对客引用资格排除内部备注", func(t *testing.T) {
		login, err := authaction.NewLoginAction(f.db).Execute(ctx, authaction.LoginInput{OrganizationID: f.owner.Organization.ID, Email: "member@navigation.test", Password: "password123"})
		if err != nil {
			t.Fatal(err)
		}
		backend := appservice.NewDirectBackend(f.db, domain.DeploymentModeSelfHosted, nil, serverfilecontent.S3Config{}, serverstorage.NewTenantResolver(f.db), nil, nil, nil, nil, nil)
		window, err := backend.ListConversationMessages(tenant.WithAccessHost(ctx, f.owner.Organization.AccessHost), appservice.RequestMeta{Token: login.Token}, f.conversationID, appservice.ConversationMessageListInput{})
		if err != nil {
			t.Fatal(err)
		}
		checked := false
		for _, message := range window.Messages {
			if message.ID != note.ID {
				continue
			}
			checked = true
			if message.CanReply || !message.CanNoteReply {
				t.Fatalf("note reply eligibility = %+v", message)
			}
		}
		if !checked {
			t.Fatal("internal note missing from member timeline")
		}
	})

	t.Run("客户不能引用内部备注", func(t *testing.T) {
		_, err := f.receive.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
			ChannelID: f.channelID, ExternalID: externalID, ConversationID: &f.conversationID,
			ClientMessageID: uuid.NewV7().String(), Body: "这是什么", ReplyToMessageID: note.ID,
		})
		var conflict *conversationaction.ConflictError
		if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonReplyTargetInvalid {
			t.Fatalf("visitor reply to note = %v", err)
		}
	})

	t.Run("同一消息编号跨可见范围冲突", func(t *testing.T) {
		changed := noteInput
		changed.Visibility = domain.MessageVisibilityShared
		_, err := send.Execute(ctx, f.member, changed)
		var conflict *conversationaction.ConflictError
		if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonIdempotencyMismatch {
			t.Fatalf("visibility change retry = %v", err)
		}
		replay, err := send.Execute(ctx, f.member, noteInput)
		if err != nil || replay.ID != note.ID {
			t.Fatalf("note replay=%+v err=%v", replay, err)
		}
	})

	t.Run("关闭周期后仍可补记内部备注", func(t *testing.T) {
		tasks := newTestTasks(f.db)
		closeSession := conversationaction.NewCloseServiceSessionAction(f.db, agentrunaction.NewExecuteAction(f.db, tasks, nil, testAttachmentReader(f.db), nil, nil), newTestTasks(f.db))
		if _, err := closeSession.Execute(ctx, f.owner, f.conversationID); err != nil {
			t.Fatal(err)
		}
		saved, err := send.Execute(ctx, f.owner, conversationaction.ServiceTextMessageInput{
			ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(),
			Body: "补一条备注", Visibility: domain.MessageVisibilityInternal,
		})
		if err != nil || saved.Visibility != domain.MessageVisibilityInternal {
			t.Fatalf("closed session note=%+v err=%v", saved, err)
		}
		// 补记不重开周期，也不推进周期摘要。
		closed := &servermodels.ServiceSession{}
		if err := f.db.NewSelect().Model(closed).Where("ss.conversation_id = ?", f.conversationID).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if closed.Status != string(domain.ServiceSessionStatusClosed) || closed.LastMessageID == saved.ID {
			t.Fatalf("closed session changed by note: %+v", closed)
		}
	})
}
