//go:build server

package server

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/appservice"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

// TestInboxActivityAppend 验证各类消息活动取数据库时钟、单调推进，并与消息一起回滚。
func TestInboxActivityAppend(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	for _, kind := range []domain.ConversationType{domain.ConversationTypeDirect, domain.ConversationTypeAgent, domain.ConversationTypeGroup, domain.ConversationTypeCustomer} {
		t.Run(string(kind), func(t *testing.T) {
			cv := &servermodels.Conversation{ID: uuid.NewV7().String(), OrganizationID: f.owner.Organization.ID, Type: string(kind), Status: "active"}
			if _, err := f.db.NewInsert().Model(cv).Column("id", "organization_id", "type", "status").Exec(ctx); err != nil {
				t.Fatal(err)
			}
			var databaseStart time.Time
			if err := f.db.NewRaw("SELECT clock_timestamp()").Scan(ctx, &databaseStart); err != nil {
				t.Fatal(err)
			}
			key := uuid.NewV7().String()
			message := &servermodels.Message{ID: uuid.NewV7().String(), OrganizationID: cv.OrganizationID, ConversationID: cv.ID, Type: "text", Body: "来源时钟超前", OriginatedAt: databaseStart.Add(24 * time.Hour), IdempotencyKey: &key}
			if err := f.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
				if err := tx.NewSelect().Model(cv).WherePK().For("UPDATE").Scan(ctx); err != nil {
					return err
				}
				_, inserted, err := chatstate.AppendMessage(ctx, tx, cv, message)
				if err == nil && !inserted {
					return errors.New("first append was not inserted")
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			var databaseEnd time.Time
			if err := f.db.NewRaw("SELECT clock_timestamp()").Scan(ctx, &databaseEnd); err != nil {
				t.Fatal(err)
			}
			if cv.LastActivityAt == nil || cv.LastActivityAt.Before(databaseStart) || cv.LastActivityAt.After(databaseEnd) {
				t.Fatalf("activity=%v database=[%v,%v]", cv.LastActivityAt, databaseStart, databaseEnd)
			}
			firstActivity := *cv.LastActivityAt
			if err := f.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
				if err := tx.NewSelect().Model(cv).WherePK().For("UPDATE").Scan(ctx); err != nil {
					return err
				}
				_, inserted, err := chatstate.AppendMessage(ctx, tx, cv, message)
				if err == nil && inserted {
					return errors.New("replay inserted a message")
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if !cv.LastActivityAt.Equal(firstActivity) {
				t.Fatal("replay moved activity")
			}
			// 模拟数据库时钟回拨前已经保存的较大活动时间。
			future := databaseStart.Add(48 * time.Hour)
			if _, err := f.db.NewUpdate().Model(cv).Set("last_activity_at = ?", future).WherePK().Exec(ctx); err != nil {
				t.Fatal(err)
			}
			if err := f.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
				if err := tx.NewSelect().Model(cv).WherePK().For("UPDATE").Scan(ctx); err != nil {
					return err
				}
				late := &servermodels.Message{ID: uuid.NewV7().String(), OrganizationID: cv.OrganizationID, ConversationID: cv.ID, Type: "system", Body: "晚到消息", OriginatedAt: databaseStart.Add(-24 * time.Hour)}
				if _, _, err := chatstate.AppendMessage(ctx, tx, cv, late); err != nil {
					return err
				}
				return chatstate.RecomputeConversationSummary(ctx, tx, cv, late.ID)
			}); err != nil {
				t.Fatal(err)
			}
			if err := f.db.NewSelect().Model(cv).WherePK().Scan(ctx); err != nil {
				t.Fatal(err)
			}
			if cv.LastActivityAt == nil || !cv.LastActivityAt.Equal(future) || cv.LastMessageAt == nil || !cv.LastMessageAt.Equal(databaseStart.Add(-24*time.Hour)) {
				t.Fatalf("late summary=%+v", cv)
			}
			// 用正常时钟再追加后回滚，检查数据库没有留下新的活动时间。
			if _, err := f.db.NewUpdate().Model(cv).Set("last_activity_at = ?", firstActivity).WherePK().Exec(ctx); err != nil {
				t.Fatal(err)
			}
			rollback := errors.New("rollback activity")
			err := f.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
				if err := tx.NewSelect().Model(cv).WherePK().For("UPDATE").Scan(ctx); err != nil {
					return err
				}
				if _, _, err := chatstate.AppendMessage(ctx, tx, cv, &servermodels.Message{ID: uuid.NewV7().String(), OrganizationID: cv.OrganizationID, ConversationID: cv.ID, Type: "text", Body: "回滚", OriginatedAt: databaseStart}); err != nil {
					return err
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatal(err)
			}
			if err := f.db.NewSelect().Model(cv).WherePK().Scan(ctx); err != nil {
				t.Fatal(err)
			}
			if cv.LastMessageSeq != 2 || !cv.LastActivityAt.Equal(firstActivity) {
				t.Fatalf("rollback summary=%+v", cv)
			}
		})
	}
}

// TestInboxSnapshot 验证列表读取后提交的新消息不会进入本次响应的未读总数。
func TestInboxSnapshot(t *testing.T) {
	f := newNavigationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	first := f.send(t, f.owner, "快照前", false)
	login, err := authaction.NewLoginAction(f.db).Execute(ctx, authaction.LoginInput{OrganizationID: f.owner.Organization.ID, Email: "member@navigation.test", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	backend := appservice.NewDirectBackend(f.db, nil, NewTenantResolver(f.db), nil, nil, nil)
	ctx = tenant.WithAccessHost(ctx, f.owner.Organization.AccessHost)
	f.db.AddQueryHook(chatQueryHook{})
	gate := newChatQueryGate(t, false, 1, func(event *bun.QueryEvent) bool {
		return event.Operation() == "SELECT" && strings.Contains(event.Query, "AS member_count") && strings.Contains(event.Query, "LIMIT 50")
	})
	var snapshot appservice.Inbox
	done := make(chan error, 1)
	go func() {
		var err error
		snapshot, err = backend.LoadInbox(context.WithValue(ctx, chatQueryGateKey{}, gate), appservice.RequestMeta{Token: login.Token}, appservice.LoadInboxInput{Scope: appservice.InboxScopeInternal})
		done <- err
	}()
	waitChatSignal(t, ctx, gate.reached)
	second := f.send(t, f.owner, "快照后提交", false)
	gate.open()
	if err := waitChatResult(t, ctx, done); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Conversations) != 1 || snapshot.Conversations[0].LastMessageID == nil || *snapshot.Conversations[0].LastMessageID != first.ID || snapshot.Conversations[0].UnreadCount != 1 || snapshot.UnreadCount != 1 || snapshot.AttentionUnreadCount != 1 {
		t.Fatalf("mixed snapshot=%+v rows=%+v", snapshot, snapshot.Conversations)
	}
	current, err := backend.LoadInbox(ctx, appservice.RequestMeta{Token: login.Token}, appservice.LoadInboxInput{Scope: appservice.InboxScopeInternal})
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Conversations) != 1 || *current.Conversations[0].LastMessageID != second.ID || current.UnreadCount != 2 || current.AttentionUnreadCount != 2 || current.Conversations[0].LastActivityAt == nil {
		t.Fatalf("next snapshot=%+v", current)
	}
}

// TestInboxActivityOrder 验证混合列表保持微秒顺序、同时间编号倒序及空会话沉底。
func TestInboxActivityOrder(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	direct, err := conversationaction.NewSendFirstDirectTextMessageAction(f.db).Execute(ctx, f.owner, conversationaction.FirstDirectTextMessageInput{TargetIdentityID: f.member.OrganizationIdentity.ID, ClientMessageID: uuid.NewV7().String(), Body: "单聊"})
	if err != nil {
		t.Fatal(err)
	}
	f.send(t, f.owner, "群聊", false)
	empty, err := conversationaction.NewCreateGroupConversationAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationInput{Title: "空群", MemberIdentityIDs: []string{f.member.OrganizationIdentity.ID}})
	if err != nil {
		t.Fatal(err)
	}
	// 客服回复使客户会话进入本人全部范围。
	if _, err := conversationaction.NewSendCustomerTextMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "回复"}); err != nil {
		t.Fatal(err)
	}
	var base time.Time
	if err := f.db.NewRaw("SELECT date_trunc('milliseconds', clock_timestamp()) - interval '1 hour'").Scan(ctx, &base); err != nil {
		t.Fatal(err)
	}
	ids := []string{direct.Conversation.ID, f.groupID, f.conversationID}
	for i, id := range ids {
		if _, err := f.db.NewUpdate().Model((*servermodels.Conversation)(nil)).Set("last_activity_at = ?", base.Add(time.Duration(2-i)*time.Microsecond)).Set("last_message_at = ?", base.Add(time.Duration(i)*time.Hour)).Where("id = ?", id).Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}
	query := inboxaction.NewLoadInboxQuery(f.db)
	rows, counts, err := query.Execute(ctx, f.owner, inboxaction.LoadInput{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, row := range rows {
		got = append(got, row.ID)
	}
	if !slices.Equal(got, append(slices.Clone(ids), empty.ID)) || counts.Unread != 0 || counts.Attention != 0 {
		t.Fatalf("order=%v counts=%+v", got, counts)
	}
	if rows[2].Customer.LastMessageAt == nil || !rows[2].Customer.LastMessageAt.Equal(base.Add(2*time.Hour)) || rows[3].LastActivityAt != nil {
		t.Fatalf("preview or empty=%+v", rows)
	}
	if _, err := f.db.NewUpdate().Model((*servermodels.Conversation)(nil)).Set("last_activity_at = ?", base).Where("id IN (?)", bun.In(ids)).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	rows, _, err = query.Execute(ctx, f.owner, inboxaction.LoadInput{})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(ids)
	slices.Reverse(ids)
	got = got[:0]
	for _, row := range rows {
		got = append(got, row.ID)
	}
	if !slices.Equal(got, append(ids, empty.ID)) {
		t.Fatalf("tie order=%v", got)
	}

	// 静音和已读改变个人投影，保留活动时间。
	if _, err := conversationaction.NewUpdateConversationNotificationSettingsAction(f.db).Execute(ctx, f.owner, f.groupID, true); err != nil {
		t.Fatal(err)
	}
	var groupMessageID string
	if err := f.db.NewSelect().Table("conversations").Column("last_message_id").Where("id = ?", f.groupID).Scan(ctx, &groupMessageID); err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewMarkConversationReadAction(f.db).Execute(ctx, f.owner, f.groupID, groupMessageID, false); err != nil {
		t.Fatal(err)
	}
	var groupActivity time.Time
	if err := f.db.NewSelect().Table("conversations").Column("last_activity_at").Where("id = ?", f.groupID).Scan(ctx, &groupActivity); err != nil || !groupActivity.Equal(base) {
		t.Fatalf("settings activity=%v err=%v", groupActivity, err)
	}

	// 从空群只改简介不会生成活动，改名追加系统消息后才进入活动区。
	if _, err := conversationaction.NewUpdateGroupConversationAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationProfileInput{ConversationID: empty.ID, Title: "空群", Description: "仅资料"}); err != nil {
		t.Fatal(err)
	}
	var activity sql.NullTime
	if err := f.db.NewSelect().Table("conversations").Column("last_activity_at").Where("id = ?", empty.ID).Scan(ctx, &activity); err != nil || activity.Valid {
		t.Fatalf("profile activity=%v err=%v", activity, err)
	}
	if _, err := conversationaction.NewUpdateGroupConversationAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationProfileInput{ConversationID: empty.ID, Title: "新群名", Description: "仅资料"}); err != nil {
		t.Fatal(err)
	}
	rows, _, err = query.Execute(ctx, f.owner, inboxaction.LoadInput{})
	if err != nil || rows[0].ID != empty.ID || rows[0].LastActivityAt == nil {
		t.Fatalf("system activity=%+v err=%v", rows, err)
	}
}

// TestInboxTelegramActivity 验证晚到的 Telegram 消息更新活动排序，保留来源时间和原客户范围。
func TestInboxTelegramActivity(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	channel, err := channelaction.NewCreateMessageChannelAction(f.db).Execute(ctx, f.owner, channelaction.CreateMessageChannelInput{
		Type: domain.ChannelTypeTelegram, Name: "活动时间验证", DefaultLocale: domain.LocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue}, FallbackTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.NewUpdate().Table("telegram_channel_settings").Set("bot_id = ?", time.Now().UnixNano()).Set("bot_token = '123:token'").Set("webhook_secret = 'secret'").Where("channel_id = ?", channel.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	var before time.Time
	if err := f.db.NewRaw("SELECT clock_timestamp()").Scan(ctx, &before); err != nil {
		t.Fatal(err)
	}
	source := before.Add(-24 * time.Hour)
	receiver := channelaction.NewReceiveTelegramWebhookAction(f.db, agentrunaction.NewScheduler(servertask.New(f.db, serverconfig.NATSConfig{})), nil, nil)
	input := channelaction.TelegramWebhookInput{Secret: "secret", UpdateID: 1, Message: &channelaction.TelegramWebhookMessage{ChatID: 12345, SenderID: 12345, MessageID: 1, DisplayName: "晚到客户", Body: "昨天的消息", OriginatedAt: source}}
	if err := receiver.Execute(ctx, channel.ID, input); err != nil {
		t.Fatal(err)
	}
	query := inboxaction.NewLoadInboxQuery(f.db)
	rows, counts, err := query.Execute(ctx, f.owner, inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewQueue})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Customer.ChannelType != domain.ChannelTypeTelegram || rows[0].LastActivityAt == nil || rows[0].LastActivityAt.Before(before) || !rows[0].Customer.LastMessageAt.Equal(source) || counts.Unread != 0 || counts.Attention != 0 {
		t.Fatalf("telegram rows=%+v counts=%+v", rows, counts)
	}
	telegram := rows[0]
	if err := receiver.Execute(ctx, channel.ID, input); err != nil {
		t.Fatal(err)
	}
	all, _, err := query.Execute(ctx, f.owner, inboxaction.LoadInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range all {
		if row.Customer != nil {
			t.Fatalf("public queue leaked into all: %+v", row)
		}
	}
	// 阅读与处理状态只改变投影，不改变活动位置。
	if _, err := conversationaction.NewMarkConversationReadAction(f.db).Execute(ctx, f.owner, telegram.ID, *telegram.LastMessageID, false); err != nil {
		t.Fatal(err)
	}
	coordinator := agentrunaction.NewExecuteAction(f.db, nil, nil)
	if _, err := conversationaction.NewCloseServiceSessionAction(f.db, coordinator).Execute(ctx, f.owner, telegram.ID); err != nil {
		t.Fatal(err)
	}
	closed, counts, err := query.Execute(ctx, f.owner, inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: domain.CustomerInboxViewClosed})
	if err != nil || len(closed) != 1 || !closed[0].LastActivityAt.Equal(*telegram.LastActivityAt) || closed[0].UnreadCount != 0 || counts.Attention != 0 {
		t.Fatalf("closed=%+v counts=%+v err=%v", closed, counts, err)
	}
	if _, err := conversationaction.NewReopenServiceSessionAction(f.db).Execute(ctx, f.owner, telegram.ID); err != nil {
		t.Fatal(err)
	}
	var activity time.Time
	if err := f.db.NewSelect().Table("conversations").Column("last_activity_at").Where("id = ?", telegram.ID).Scan(ctx, &activity); err != nil || !activity.Equal(*telegram.LastActivityAt) {
		t.Fatalf("reopened=%v err=%v", activity, err)
	}
}
