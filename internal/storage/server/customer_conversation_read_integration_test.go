//go:build server

package server

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type customerReadFixture struct {
	navigationFixture
	channelID, conversationID string
	receive                   *conversationaction.ReceiveWebsiteCustomerTextMessageAction
}

// newCustomerReadFixture 建立两个客服共享的网站客户会话。
func newCustomerReadFixture(t *testing.T) customerReadFixture {
	t.Helper()
	f := newNavigationFixture(t)
	ctx := context.Background()
	var roleID string
	if err := f.db.NewSelect().Table("roles").Column("id").Where("organization_id = ? AND kind = ?", f.owner.Organization.ID, domain.RoleKindCustomerService).Scan(ctx, &roleID); err != nil {
		t.Fatal(err)
	}
	for _, identity := range []*servermodels.Identity{f.owner, f.member} {
		if _, err := f.db.NewUpdate().Table("organization_identities").Set("role_id = ?", roleID).Where("organization_id = ? AND id = ?", identity.Organization.ID, identity.OrganizationIdentity.ID).Exec(ctx); err != nil {
			t.Fatal(err)
		}
		identity.OrganizationIdentity.RoleID = roleID
	}
	channel, err := channelaction.NewCreateMessageChannelAction(f.db).Execute(ctx, f.owner, channelaction.CreateMessageChannelInput{
		Type: domain.ChannelTypeWebsite, Name: "客服未读测试", DefaultLocale: domain.LocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
	})
	if err != nil {
		t.Fatal(err)
	}
	receive := conversationaction.NewReceiveWebsiteCustomerTextMessageAction(f.db, agentrunaction.NewScheduler(servertask.New(f.db, serverconfig.NATSConfig{})))
	result, err := receive.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: channel.ID, ExternalID: "web-session:0123456789abcdef0123456789abcdef", ClientMessageID: uuid.NewV7().String(), Body: "客户首条消息",
	})
	if err != nil {
		t.Fatal(err)
	}
	return customerReadFixture{navigationFixture: f, channelID: channel.ID, conversationID: result.Conversation.ID, receive: receive}
}

// inboxRow 读取指定客服视图并核对内部提醒总数没有被客户消息改变。
func (f customerReadFixture) inboxRow(t *testing.T, identity *servermodels.Identity, view domain.CustomerInboxView) inboxaction.ConversationSummary {
	t.Helper()
	rows, counts, err := inboxaction.NewLoadInboxQuery(f.db).Execute(context.Background(), identity, inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: view})
	if err != nil {
		t.Fatal(err)
	}
	if counts.Unread != 0 || counts.Attention != 0 {
		t.Fatalf("customer changed internal counts: %+v", counts)
	}
	for _, row := range rows {
		if row.ID == f.conversationID {
			return row
		}
	}
	t.Fatalf("customer conversation missing: %+v", rows)
	return inboxaction.ConversationSummary{}
}

// visitorMessage 通过网站入口向已有客服会话发送消息。
func (f customerReadFixture) visitorMessage(ctx context.Context, body string) (conversationaction.ReceiveWebsiteCustomerTextMessageResult, error) {
	return f.receive.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: f.channelID, ExternalID: "web-session:0123456789abcdef0123456789abcdef", ConversationID: &f.conversationID,
		ClientMessageID: uuid.NewV7().String(), Body: body,
	})
}

// TestCustomerConversationPersonalRead 验证独立阅读、自己的回复及跨处理周期的水位。
func TestCustomerConversationPersonalRead(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	read := conversationaction.NewMarkConversationReadAction(f.db)
	first := f.inboxRow(t, f.owner, domain.CustomerInboxViewQueue)
	if first.UnreadCount != 1 || first.LastReadMessageID != nil {
		t.Fatalf("initial unread: %+v", first)
	}
	if _, err := read.Execute(ctx, f.owner, f.conversationID, *first.LastMessageID, true); err != nil {
		t.Fatal(err)
	}
	if row := f.inboxRow(t, f.owner, domain.CustomerInboxViewQueue); row.UnreadCount != 0 {
		t.Fatalf("owner unread: %+v", row)
	}
	if row := f.inboxRow(t, f.member, domain.CustomerInboxViewQueue); row.UnreadCount != 1 {
		t.Fatalf("reading changed coworker: %+v", row)
	}
	// 阅读不会把旁观客服加入参与者或领取公共队列。
	count, err := f.db.NewSelect().Table("conversation_participants").Where("conversation_id = ?", f.conversationID).Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("reading created participant: %d %v", count, err)
	}
	reply, err := conversationaction.NewSendCustomerTextMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "客服回复"})
	if err != nil {
		t.Fatal(err)
	}
	if row := f.inboxRow(t, f.owner, domain.CustomerInboxViewMine); row.UnreadCount != 0 {
		t.Fatalf("own reply unread: %+v", row)
	}
	if row := f.inboxRow(t, f.member, domain.CustomerInboxViewCoworkers); row.UnreadCount != 2 {
		t.Fatalf("coworker reply not counted: %+v", row)
	}
	for _, id := range []string{reply.ID, *first.LastMessageID, reply.ID} {
		state, err := read.Execute(ctx, f.member, f.conversationID, id, false)
		if err != nil || state.LastReadMessageID != reply.ID {
			t.Fatalf("monotonic read: %+v %v", state, err)
		}
	}
	coordinator := agentrunaction.NewExecuteAction(f.db, nil, nil)
	if _, err := conversationaction.NewTransferServiceSessionAction(f.db, coordinator, agentrunaction.NewScheduler(servertask.New(f.db, serverconfig.NATSConfig{}))).Execute(ctx, f.owner, conversationaction.TransferServiceSessionInput{ConversationID: f.conversationID, AssigneeIdentityID: f.member.OrganizationIdentity.ID}); err != nil {
		t.Fatal(err)
	}
	if row := f.inboxRow(t, f.owner, domain.CustomerInboxViewCoworkers); row.LastReadMessageID == nil || *row.LastReadMessageID != *first.LastMessageID {
		t.Fatalf("transfer changed owner read: %+v", row)
	}
	if _, err := conversationaction.NewCloseServiceSessionAction(f.db, coordinator).Execute(ctx, f.member, f.conversationID); err != nil {
		t.Fatal(err)
	}
	if row := f.inboxRow(t, f.member, domain.CustomerInboxViewClosed); row.UnreadCount != 0 || *row.LastReadMessageID != reply.ID {
		t.Fatalf("close changed read: %+v", row)
	}
	if _, err := conversationaction.NewReopenServiceSessionAction(f.db).Execute(ctx, f.member, f.conversationID); err != nil {
		t.Fatal(err)
	}
	if row := f.inboxRow(t, f.member, domain.CustomerInboxViewMine); *row.LastReadMessageID != reply.ID {
		t.Fatalf("reopen changed read: %+v", row)
	}
	if _, err := conversationaction.NewCloseServiceSessionAction(f.db, coordinator).Execute(ctx, f.member, f.conversationID); err != nil {
		t.Fatal(err)
	}
	next, err := f.visitorMessage(ctx, "新的处理周期")
	if err != nil || !next.OpenedNewServiceSession {
		t.Fatalf("new session: %+v %v", next, err)
	}
	if row := f.inboxRow(t, f.member, domain.CustomerInboxViewQueue); row.UnreadCount != 1 || *row.LastReadMessageID != reply.ID {
		t.Fatalf("new session reset read: %+v", row)
	}
}

// TestCustomerConversationReadBoundaries 验证企业隔离、消息归属、删除和发送主体判断。
func TestCustomerConversationReadBoundaries(t *testing.T) {
	f := newCustomerReadFixture(t)
	other := newCustomerReadFixture(t)
	ctx := context.Background()
	read := conversationaction.NewMarkConversationReadAction(f.db)
	first := f.inboxRow(t, f.owner, domain.CustomerInboxViewQueue)
	foreign := other.inboxRow(t, other.owner, domain.CustomerInboxViewQueue)
	for _, attempt := range []struct {
		identity              *servermodels.Identity
		conversation, message string
	}{
		{other.owner, f.conversationID, *first.LastMessageID},
		{f.owner, f.conversationID, *foreign.LastMessageID},
		{f.owner, f.conversationID, uuid.NewV7().String()},
	} {
		if _, err := read.Execute(ctx, attempt.identity, attempt.conversation, attempt.message, true); !errors.Is(err, conversationaction.ErrConversationNotFound) {
			t.Fatalf("invalid read: %v", err)
		}
	}
	// 联系人与员工即使来源编号相同，也必须按主体类型区分。
	if _, err := f.db.NewUpdate().Table("chat_subjects").Set("source_id = ?", f.owner.OrganizationIdentity.ID).Where("organization_id = ? AND kind = ?", f.owner.Organization.ID, domain.ChatSubjectKindContact).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if row := f.inboxRow(t, f.owner, domain.CustomerInboxViewQueue); row.UnreadCount != 1 {
		t.Fatalf("contact mistaken for self: %+v", row)
	}
	if _, err := read.Execute(ctx, f.owner, f.conversationID, *first.LastMessageID, false); err != nil {
		t.Fatal(err)
	}
	second, err := f.visitorMessage(ctx, "会被删除的消息")
	if err != nil {
		t.Fatal(err)
	}
	third, err := f.visitorMessage(ctx, "保留的消息")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.NewUpdate().Table("messages").Set("deleted_at = now()").Where("id IN (?)", bun.In([]string{*first.LastMessageID, second.Message.ID})).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if row := f.inboxRow(t, f.owner, domain.CustomerInboxViewQueue); row.UnreadCount != 1 || *row.LastReadMessageID != *first.LastMessageID {
		t.Fatalf("deleted read anchor: %+v", row)
	}
	if _, err := read.Execute(ctx, f.owner, f.conversationID, third.Message.ID, true); err != nil {
		t.Fatal(err)
	}
	if row := f.inboxRow(t, f.owner, domain.CustomerInboxViewQueue); row.UnreadCount != 0 {
		t.Fatalf("explicit read: %+v", row)
	}
}

// TestCustomerConversationDelayedMessage 验证等待其他锁的消息在后续提交时仍进入未读与增量历史。
func TestCustomerConversationDelayedMessage(t *testing.T) {
	for _, visitor := range []bool{true, false} {
		name := "member"
		if visitor {
			name = "visitor"
		}
		t.Run(name, func(t *testing.T) {
			f := newCustomerReadFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			tx, err := f.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			var blockerPID int
			if err := tx.NewSelect().ColumnExpr("pg_backend_pid()").Scan(ctx, &blockerPID); err != nil {
				t.Fatal(err)
			}
			if visitor {
				_, err = tx.ExecContext(ctx, "SELECT id FROM contact_channel_identities WHERE channel_id = ? FOR UPDATE", f.channelID)
			} else {
				_, err = tx.ExecContext(ctx, "SELECT id FROM users WHERE id = ? FOR UPDATE", f.owner.User.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				if visitor {
					_, err := f.visitorMessage(ctx, "等待后入站")
					done <- err
				} else {
					_, err := conversationaction.NewSendCustomerTextMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "等待后回复"})
					done <- err
				}
			}()
			// 等到请求确实阻塞，再让另一条消息先提交并被阅读。
			for {
				var blocked bool
				if err := f.db.NewSelect().ColumnExpr("EXISTS (SELECT 1 FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid)))", blockerPID).Scan(ctx, &blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				select {
				case err := <-done:
					t.Fatalf("message finished before lock: %v", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(10 * time.Millisecond):
				}
			}
			var earlierID string
			if visitor {
				reply, err := conversationaction.NewSendCustomerTextMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "先提交回复"})
				if err != nil {
					t.Fatal(err)
				}
				earlierID = reply.ID
			} else {
				inbound, err := f.visitorMessage(ctx, "先提交入站")
				if err != nil {
					t.Fatal(err)
				}
				earlierID = inbound.Message.ID
			}
			if _, err := conversationaction.NewMarkConversationReadAction(f.db).Execute(ctx, f.member, f.conversationID, earlierID, false); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			row := f.inboxRow(t, f.member, domain.CustomerInboxViewCoworkers)
			if row.UnreadCount != 1 {
				t.Fatalf("late commit disappeared from unread: %+v", row)
			}
			var earlier servermodels.Message
			if err := f.db.NewSelect().Model(&earlier).Where("msg.id = ?", earlierID).Scan(ctx); err != nil {
				t.Fatal(err)
			}
			history, err := conversationaction.NewListConversationMessagesQuery(f.db).Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID, After: &conversationaction.MessageCursorPoint{ID: earlier.ID, OriginatedAt: earlier.OriginatedAt, SourceOrder: earlier.SourceOrder}})
			if err != nil || len(history.Messages) != 1 {
				t.Fatalf("late commit missing from after: %+v %v", history, err)
			}
		})
	}
}
