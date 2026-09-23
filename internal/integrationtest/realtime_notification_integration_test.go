//go:build server

package integrationtest

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/nats-io/nats.go"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	telegramintegration "github.com/runforyou-ai/cervi/internal/integration/telegram"
	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/servertest"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// receivedNotification 表示订阅端收到的一条实时通知。
type receivedNotification struct {
	Subject         string
	Kind            string
	ConversationID  string
	Version         string
	SenderSubjectID string
	Active          bool
	// ServiceSessionID 与 AttentionReason 只由客服处理周期提醒携带。
	ServiceSessionID string
	AttentionReason  string
}

// realtimeFeed 订阅单个测试企业的全部受众通知。
type realtimeFeed struct {
	namespace      string
	organizationID string
	subscription   *nats.Subscription
}

// startRealtimeFeed 启动独立命名空间的实时发布器，并订阅指定企业的全部受众通知。
func startRealtimeFeed(t *testing.T, organizationID string) *realtimeFeed {
	t.Helper()
	config := servertest.NATSConfig(t, "test_realtime_"+strings.ReplaceAll(uuid.NewV7().String(), "-", ""))
	publisher := realtime.NewPublisher(config)
	if err := publisher.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = publisher.Stop() })
	connection, err := nats.Connect(config.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connection.Close)
	subscription, err := connection.SubscribeSync("cervi." + config.Namespace + ".realtime." + organizationID + ".>")
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Flush(); err != nil {
		t.Fatal(err)
	}
	return &realtimeFeed{namespace: config.Namespace, organizationID: organizationID, subscription: subscription}
}

// notice 构造发往指定用户受众的期望通知。
func (f *realtimeFeed) notice(userID string, kind realtime.Kind, conversationID string, version int64) receivedNotification {
	return receivedNotification{
		Subject: realtime.Subject(f.namespace, f.organizationID, realtime.AudienceUser, userID),
		Kind:    string(kind), ConversationID: conversationID, Version: strconv.FormatInt(version, 10),
	}
}

// userTyping 构造发往指定用户受众的会话输入状态。
func (f *realtimeFeed) userTyping(userID, conversationID, senderSubjectID string, active bool) receivedNotification {
	return receivedNotification{
		Subject: realtime.Subject(f.namespace, f.organizationID, realtime.AudienceUser, userID),
		Kind:    string(realtime.KindConversationTyping), ConversationID: conversationID, SenderSubjectID: senderSubjectID, Active: active,
	}
}

// loadIdentitySubjectID 读取企业身份的聊天主体编号。
func loadIdentitySubjectID(t *testing.T, db *bun.DB, organizationID, identityID string) string {
	t.Helper()
	var subjectID string
	if err := db.NewSelect().Table("chat_subjects").Column("id").
		Where("organization_id = ? AND kind = ? AND source_id = ?", organizationID, domain.ChatSubjectKindOrganizationIdentity, identityID).
		Scan(context.Background(), &subjectID); err != nil {
		t.Fatal(err)
	}
	return subjectID
}

// removed 构造发往指定用户受众的会话失权通知。
func (f *realtimeFeed) removed(userID, conversationID string) receivedNotification {
	return receivedNotification{
		Subject: realtime.Subject(f.namespace, f.organizationID, realtime.AudienceUser, userID),
		Kind:    string(realtime.KindConversationRemoved), ConversationID: conversationID,
	}
}

// customerInbox 构造发往企业客服共享受众的客户会话变更通知。
func (f *realtimeFeed) customerInbox(conversationID string, version int64) receivedNotification {
	return receivedNotification{
		Subject: realtime.Subject(f.namespace, f.organizationID, realtime.AudienceCustomerInbox, f.organizationID),
		Kind:    string(realtime.KindConversationChanged), ConversationID: conversationID, Version: strconv.FormatInt(version, 10),
	}
}

// visitorDirectory 构造发往网站渠道身份受众的客户线程变更通知。
func (f *realtimeFeed) visitorDirectory(channelIdentityID, conversationID string, version int64) receivedNotification {
	return receivedNotification{
		Subject: realtime.Subject(f.namespace, f.organizationID, realtime.AudienceVisitorDirectory, channelIdentityID),
		Kind:    string(realtime.KindConversationChanged), ConversationID: conversationID, Version: strconv.FormatInt(version, 10),
	}
}

// customerInboxTyping 构造发往企业客服共享受众的客户会话输入状态。
func (f *realtimeFeed) customerInboxTyping(conversationID, senderSubjectID string, active bool) receivedNotification {
	return receivedNotification{
		Subject: realtime.Subject(f.namespace, f.organizationID, realtime.AudienceCustomerInbox, f.organizationID),
		Kind:    string(realtime.KindConversationTyping), ConversationID: conversationID, SenderSubjectID: senderSubjectID, Active: active,
	}
}

// visitorTyping 构造发往网站渠道身份受众的访客可见输入状态。
func (f *realtimeFeed) visitorTyping(channelIdentityID, conversationID string, active bool) receivedNotification {
	return receivedNotification{
		Subject: realtime.Subject(f.namespace, f.organizationID, realtime.AudienceVisitorDirectory, channelIdentityID),
		Kind:    string(realtime.KindVisitorTyping), ConversationID: conversationID, Active: active,
	}
}

// next 读取并解析下一条实时通知。
func (f *realtimeFeed) next(t *testing.T) (receivedNotification, error) {
	t.Helper()
	message, err := f.subscription.NextMsg(5 * time.Second)
	if err != nil {
		return receivedNotification{}, err
	}
	var fields map[string]any
	if err := json.Unmarshal(message.Data, &fields); err != nil {
		t.Fatalf("解析实时通知 %s: %v", message.Data, err)
	}
	// 载荷只允许种类、会话 ID、版本、发送者主体、输入状态与客服处理周期提醒。
	for key := range fields {
		if key != "kind" && key != "conversationId" && key != "version" && key != "senderSubjectId" && key != "active" &&
			key != "serviceSessionId" && key != "attentionReason" {
			t.Fatalf("实时通知含业务字段: %s", message.Data)
		}
	}
	text := func(key string) string {
		value, _ := fields[key].(string)
		return value
	}
	active, _ := fields["active"].(bool)
	return receivedNotification{
		Subject: message.Subject, Kind: text("kind"), ConversationID: text("conversationId"),
		Version: text("version"), SenderSubjectID: text("senderSubjectId"), Active: active,
		ServiceSessionID: text("serviceSessionId"), AttentionReason: text("attentionReason"),
	}, nil
}

// expectTyping 只比较输入状态通知，读取期间忽略其他种类的通知。
func (f *realtimeFeed) expectTyping(t *testing.T, want ...receivedNotification) {
	t.Helper()
	got := make([]receivedNotification, 0, len(want))
	for len(got) < len(want) {
		notification, err := f.next(t)
		if err != nil {
			t.Fatalf("等待输入状态通知: %v，已收到 %+v", err, got)
		}
		if notification.Kind == string(realtime.KindConversationTyping) || notification.Kind == string(realtime.KindVisitorTyping) {
			got = append(got, notification)
		}
	}
	compareNotifications(t, got, want)
}

// expectTypingStopped 读取输入状态直到收到指定的停止输入，其间只允许同一发送者的正在输入刷新。
func (f *realtimeFeed) expectTypingStopped(t *testing.T, want receivedNotification) {
	t.Helper()
	refresh := want
	refresh.Active = true
	for {
		notification, err := f.next(t)
		if err != nil {
			t.Fatalf("等待停止输入通知: %v", err)
		}
		if notification.Kind != string(realtime.KindConversationTyping) && notification.Kind != string(realtime.KindVisitorTyping) {
			continue
		}
		if notification == want {
			return
		}
		if notification != refresh {
			t.Fatalf("输入状态不符 got=%+v want=%+v", notification, want)
		}
	}
}

// expectNoTyping 在短暂等待内确认没有输入状态通知，其他通知忽略。
func (f *realtimeFeed) expectNoTyping(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		message, err := f.subscription.NextMsg(remaining)
		if errors.Is(err, nats.ErrTimeout) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(message.Data), `"kind":"`+string(realtime.KindConversationTyping)+`"`) {
			t.Fatalf("收到不应送达的输入状态 %s", message.Data)
		}
	}
}

// expect 读取与期望数量相同的通知，并与期望集合按任意顺序比较。
func (f *realtimeFeed) expect(t *testing.T, want ...receivedNotification) {
	t.Helper()
	got := make([]receivedNotification, 0, len(want))
	for range want {
		notification, err := f.next(t)
		if err != nil {
			t.Fatalf("等待实时通知: %v，已收到 %+v", err, got)
		}
		got = append(got, notification)
	}
	compareNotifications(t, got, want)
}

// compareNotifications 按任意顺序比较收到与期望的通知集合。
func compareNotifications(t *testing.T, got, want []receivedNotification) {
	t.Helper()
	compare := func(a, b receivedNotification) int {
		return strings.Compare(
			a.Subject+a.Kind+a.ConversationID+a.Version+a.SenderSubjectID+strconv.FormatBool(a.Active)+a.ServiceSessionID+a.AttentionReason,
			b.Subject+b.Kind+b.ConversationID+b.Version+b.SenderSubjectID+strconv.FormatBool(b.Active)+b.ServiceSessionID+b.AttentionReason,
		)
	}
	slices.SortFunc(got, compare)
	slices.SortFunc(want, compare)
	if !slices.Equal(got, want) {
		t.Fatalf("实时通知不符\ngot=%+v\nwant=%+v", got, want)
	}
}

// loadChannelIdentityID 读取客户会话所属的渠道身份记录 ID。
func loadChannelIdentityID(t *testing.T, db *bun.DB, conversationID string) string {
	t.Helper()
	var value string
	if err := db.NewSelect().Table("customer_conversations").Column("contact_channel_identity_id").Where("conversation_id = ?", conversationID).Scan(context.Background(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

// loadConversationStateVersion 读取用户个人会话状态版本。
func loadConversationStateVersion(t *testing.T, db *bun.DB, conversationID, userID string) int64 {
	t.Helper()
	var version int64
	if err := db.NewSelect().Table("conversation_user_states").Column("version").Where("conversation_id = ? AND user_id = ?", conversationID, userID).Scan(context.Background(), &version); err != nil {
		t.Fatal(err)
	}
	return version
}

// loadProfileVersion 读取用户身份资料版本。
func loadProfileVersion(t *testing.T, db *bun.DB, userID string) int64 {
	t.Helper()
	var version int64
	if err := db.NewSelect().Table("users").Column("profile_version").Where("id = ?", userID).Scan(context.Background(), &version); err != nil {
		t.Fatal(err)
	}
	return version
}

// TestRealtimeConversationNotifications 验证消息通知真人成员、同事务合并多次变化，回滚不发布。
func TestRealtimeConversationNotifications(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	second, err := conversationaction.NewCreateGroupConversationAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationInput{Title: "第二个群", MemberIdentityIDs: []string{f.member.OrganizationIdentity.ID}})
	if err != nil {
		t.Fatal(err)
	}
	feed := startRealtimeFeed(t, f.owner.Organization.ID)

	// 群消息通知全部真人成员，发送者另收阅读水位推进。
	f.send(t, f.owner, "实时通知", false)
	groupVersion := loadConversationVersion(t, f.db, f.groupID)
	feed.expect(t,
		feed.notice(f.owner.User.ID, realtime.KindConversationChanged, f.groupID, groupVersion),
		feed.notice(f.member.User.ID, realtime.KindConversationChanged, f.groupID, groupVersion),
		feed.notice(f.owner.User.ID, realtime.KindConversationStateChanged, f.groupID, loadConversationStateVersion(t, f.db, f.groupID, f.owner.User.ID)),
	)

	errRollback := errors.New("rollback")
	// appendMessages 在同一事务内按顺序向会话追加消息。
	appendMessages := func(fail bool, conversationIDs ...string) error {
		return realtime.RunInTx(ctx, f.db, func(ctx context.Context, tx bun.Tx) error {
			for _, conversationID := range conversationIDs {
				conversation, err := chatstate.LockConversation(ctx, tx, f.owner.Organization.ID, conversationID)
				if err != nil {
					return err
				}
				message := &servermodels.Message{ID: uuid.NewV7().String(), OrganizationID: f.owner.Organization.ID, ConversationID: conversationID, Type: string(domain.MessageTypeText), Body: "事务通知", OriginatedAt: time.Now().UTC()}
				if _, _, err := chatstate.AppendMessage(ctx, tx, conversation, message); err != nil {
					return err
				}
			}
			if fail {
				return errRollback
			}
			return nil
		})
	}
	if err := appendMessages(true, f.groupID); !errors.Is(err, errRollback) {
		t.Fatalf("rollback err=%v", err)
	}
	if version := loadConversationVersion(t, f.db, f.groupID); version != groupVersion {
		t.Fatalf("rollback version=%d want=%d", version, groupVersion)
	}
	// 同一事务写两个会话各得一条通知，同会话两次追加只保留最高版本。
	if err := appendMessages(false, f.groupID, second.ID, f.groupID); err != nil {
		t.Fatal(err)
	}
	groupVersion = loadConversationVersion(t, f.db, f.groupID)
	secondVersion := loadConversationVersion(t, f.db, second.ID)
	feed.expect(t,
		feed.notice(f.owner.User.ID, realtime.KindConversationChanged, f.groupID, groupVersion),
		feed.notice(f.member.User.ID, realtime.KindConversationChanged, f.groupID, groupVersion),
		feed.notice(f.owner.User.ID, realtime.KindConversationChanged, second.ID, secondVersion),
		feed.notice(f.member.User.ID, realtime.KindConversationChanged, second.ID, secondVersion),
	)
	// 以一次本人静音收尾。
	if _, err := conversationaction.NewUpdateConversationNotificationSettingsAction(f.db).Execute(ctx, f.owner, second.ID, true); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, feed.notice(f.owner.User.ID, realtime.KindConversationStateChanged, second.ID, loadConversationStateVersion(t, f.db, second.ID, f.owner.User.ID)))
}

// TestRealtimeConversationStateNotifications 验证已读、静音、手动未读与提及确认只通知本人，重复操作不发布。
func TestRealtimeConversationStateNotifications(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	last := f.send(t, f.owner, "待读", false)
	mention := f.send(t, f.owner, "提及", false, f.subjectID)
	feed := startRealtimeFeed(t, f.owner.Organization.ID)
	read := conversationaction.NewMarkConversationReadAction(f.db)
	mute := conversationaction.NewUpdateConversationNotificationSettingsAction(f.db)
	mark := conversationaction.NewUpdateConversationUnreadMarkAction(f.db)
	review := conversationaction.NewMarkConversationMentionReviewedAction(f.db)
	for _, step := range []struct {
		name   string
		change func() error
	}{
		{"已读", func() error { _, err := read.Execute(ctx, f.member, f.groupID, last.ID, false); return err }},
		{"静音", func() error { _, err := mute.Execute(ctx, f.member, f.groupID, true); return err }},
		{"手动未读", func() error { return mark.Execute(ctx, f.member, f.groupID, true) }},
		{"标为已读清除未读标记", func() error { _, err := read.Execute(ctx, f.member, f.groupID, mention.ID, true); return err }},
		{"确认提及", func() error { _, err := review.Execute(ctx, f.member, f.groupID, mention.ID); return err }},
		{"取消静音", func() error { _, err := mute.Execute(ctx, f.member, f.groupID, false); return err }},
	} {
		// 首次操作通知本人，重复同一操作不发布。
		for attempt := range 2 {
			if err := step.change(); err != nil {
				t.Fatalf("%s%d: %v", step.name, attempt, err)
			}
			if attempt == 0 {
				feed.expect(t, feed.notice(f.member.User.ID, realtime.KindConversationStateChanged, f.groupID, loadConversationStateVersion(t, f.db, f.groupID, f.member.User.ID)))
			}
		}
	}
	if _, err := mute.Execute(ctx, f.member, f.groupID, true); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, feed.notice(f.member.User.ID, realtime.KindConversationStateChanged, f.groupID, loadConversationStateVersion(t, f.db, f.groupID, f.member.User.ID)))
}

// TestRealtimeIdentityProfileNotifications 验证身份资料与账户偏好实际变化时只通知资料所属用户。
func TestRealtimeIdentityProfileNotifications(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	feed := startRealtimeFeed(t, f.owner.Organization.ID)
	workStatus := useraction.NewUpdateWorkStatusAction(f.db, newTestTasks(f.db))
	preferences := useraction.NewUpdatePreferencesAction(f.db)
	updateUser := useraction.NewUpdateUserAction(f.db, testServiceSessionReturner(f.db), newTestTasks(f.db))
	for _, step := range []struct {
		name   string
		change func() error
	}{
		{"工作状态", func() error {
			_, err := workStatus.Execute(ctx, f.member, useraction.WorkStatusInput{WorkStatus: domain.WorkStatusAway})
			return err
		}},
		{"账户偏好", func() error {
			_, err := preferences.Execute(ctx, f.member, useraction.PreferencesInput{Locale: domain.Locale(f.member.User.Locale), TimeZone: "Asia/Shanghai", MessageNotificationsEnabled: f.member.User.MessageNotificationsEnabled})
			return err
		}},
		{"管理员修改邮箱", func() error {
			_, err := updateUser.Execute(ctx, f.owner, f.member.User.ID, useraction.UpdateInput{DisplayName: "成员", Email: "renamed@navigation.test", RoleID: f.member.User.RoleID})
			return err
		}},
	} {
		// 首次保存通知资料所属用户，重复保存不发布。
		for attempt := range 2 {
			if err := step.change(); err != nil {
				t.Fatalf("%s%d: %v", step.name, attempt, err)
			}
			if attempt == 0 {
				feed.expect(t, feed.notice(f.member.User.ID, realtime.KindIdentityProfileChanged, "", loadProfileVersion(t, f.db, f.member.User.ID)))
			}
		}
	}
	if _, err := workStatus.Execute(ctx, f.member, useraction.WorkStatusInput{WorkStatus: domain.WorkStatusOffDuty}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, feed.notice(f.member.User.ID, realtime.KindIdentityProfileChanged, "", loadProfileVersion(t, f.db, f.member.User.ID)))
}

// TestRealtimeAttachmentMessageNotification 验证附件消息保存后通知单聊双方，发送者另收阅读水位推进。
func TestRealtimeAttachmentMessageNotification(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	fileID := uploadedAttachment(t, f.db, f.owner, "photo.png", "image/png")
	feed := startRealtimeFeed(t, f.owner.Organization.ID)
	result, err := conversationaction.NewSendAttachmentMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.AttachmentMessageInput{
		TargetIdentityID: f.member.OrganizationIdentity.ID, ClientMessageID: uuid.NewV7().String(), FileID: fileID,
	})
	if err != nil {
		t.Fatal(err)
	}
	version := loadConversationVersion(t, f.db, result.ConversationID)
	feed.expect(t,
		feed.notice(f.owner.User.ID, realtime.KindConversationChanged, result.ConversationID, version),
		feed.notice(f.member.User.ID, realtime.KindConversationChanged, result.ConversationID, version),
		feed.notice(f.owner.User.ID, realtime.KindConversationStateChanged, result.ConversationID, loadConversationStateVersion(t, f.db, result.ConversationID, f.owner.User.ID)),
	)
	// 以一次本人静音收尾。
	if _, err := conversationaction.NewUpdateConversationNotificationSettingsAction(f.db).Execute(ctx, f.owner, result.ConversationID, true); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, feed.notice(f.owner.User.ID, realtime.KindConversationStateChanged, result.ConversationID, loadConversationStateVersion(t, f.db, result.ConversationID, f.owner.User.ID)))
}

// TestRealtimeNotificationsWithoutNATS 验证 NATS 不可用时业务写入照常成功，探针仍反映变化。
func TestRealtimeNotificationsWithoutNATS(t *testing.T) {
	f := newNavigationFixture(t)
	publisher := realtime.NewPublisher(serverconfig.NATSConfig{URL: "nats://127.0.0.1:1", Namespace: "test_realtime_unavailable"})
	if err := publisher.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = publisher.Stop() })
	before := loadSyncHeads(t, f.db, f.member)
	f.send(t, f.owner, "NATS 不可用", false)
	if after := loadSyncHeads(t, f.db, f.member); after.ConversationChecksum == before.ConversationChecksum {
		t.Fatalf("checksum unchanged before=%+v after=%+v", before, after)
	}
}

// TestRealtimeGroupMembershipNotifications 验证群创建、资料与成员关系变化通知变更前后的真人受众，被移出者只收会话失权通知。
func TestRealtimeGroupMembershipNotifications(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	third := newChatLockUser(t, f.db, f.owner)
	coordinator := newGroupAgentCoordinator(f.db)
	feed := startRealtimeFeed(t, f.owner.Organization.ID)

	group, err := conversationaction.NewCreateGroupConversationAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationInput{
		Title: "成员变化群", MemberIdentityIDs: []string{f.member.OrganizationIdentity.ID, third.OrganizationIdentity.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	// changed 构造指定成员在群当前版本上的会话变更通知。
	changed := func(members ...*servermodels.Identity) []receivedNotification {
		version := loadConversationVersion(t, f.db, group.ID)
		notices := make([]receivedNotification, 0, len(members))
		for _, member := range members {
			notices = append(notices, feed.notice(member.User.ID, realtime.KindConversationChanged, group.ID, version))
		}
		return notices
	}
	// actorRead 构造系统事件推进操作人阅读水位后的本人会话状态通知。
	actorRead := func(actor *servermodels.Identity) receivedNotification {
		return feed.notice(actor.User.ID, realtime.KindConversationStateChanged, group.ID, loadConversationStateVersion(t, f.db, group.ID, actor.User.ID))
	}

	// 建群以初始版本通知全部真人成员。
	if version := loadConversationVersion(t, f.db, group.ID); version != 1 {
		t.Fatalf("created group version=%d", version)
	}
	feed.expect(t, changed(f.owner, f.member, third)...)

	// 只改简介不追加系统消息，仍按新版本通知全部成员。
	if _, err := conversationaction.NewUpdateGroupConversationAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationProfileInput{
		ConversationID: group.ID, Title: "成员变化群", Description: "只改简介",
	}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed(f.owner, f.member, third)...)

	// 改名同时推进资料版本并追加系统事件，同一事务的通知合并为最高版本。
	if _, err := conversationaction.NewUpdateGroupConversationAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationProfileInput{
		ConversationID: group.ID, Title: "改名后的群", Description: "只改简介",
	}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, append(changed(f.owner, f.member, third), actorRead(f.owner))...)

	// 移出成员后仍在群内的成员收到变更，被移出者只收到失权通知。
	if _, err := conversationaction.NewRemoveGroupConversationMemberAction(f.db, coordinator).Execute(ctx, f.owner, conversationaction.GroupConversationMemberInput{
		ConversationID: group.ID, MemberIdentityID: third.OrganizationIdentity.ID,
	}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, append(changed(f.owner, f.member), feed.removed(third.User.ID, group.ID), actorRead(f.owner))...)

	// 重新加入是新的有效关系，重入者与原成员一起收到变更。
	if _, err := conversationaction.NewAddGroupConversationMembersAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationMembersInput{
		ConversationID: group.ID, MemberIdentityIDs: []string{third.OrganizationIdentity.ID},
	}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, append(changed(f.owner, f.member, third), actorRead(f.owner))...)

	// 主动退出的成员收到失权通知，并作为系统事件操作人收到本人阅读水位通知；其余成员收到变更。
	if err := conversationaction.NewLeaveGroupConversationAction(f.db, newGroupAgentCoordinator(f.db)).Execute(ctx, third, group.ID); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, append(changed(f.owner, f.member), feed.removed(third.User.ID, group.ID), actorRead(third))...)

	// 转让群主通知全部当前成员。
	if _, err := conversationaction.NewTransferGroupConversationOwnerAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationOwnerInput{
		ConversationID: group.ID, OwnerIdentityID: f.member.OrganizationIdentity.ID,
	}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, append(changed(f.owner, f.member), actorRead(f.owner))...)

	// 失去管理资格的移除失败，不留系统消息也不发布通知。
	messageCount, err := f.db.NewSelect().Model((*servermodels.Message)(nil)).Where("conversation_id = ?", group.ID).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conversationaction.NewRemoveGroupConversationMemberAction(f.db, coordinator).Execute(ctx, f.owner, conversationaction.GroupConversationMemberInput{
		ConversationID: group.ID, MemberIdentityID: f.member.OrganizationIdentity.ID,
	}); err == nil {
		t.Fatal("former owner removed the new owner")
	}
	if count, err := f.db.NewSelect().Model((*servermodels.Message)(nil)).Where("conversation_id = ?", group.ID).Count(ctx); err != nil || count != messageCount {
		t.Fatalf("failed removal messages=%d want=%d err=%v", count, messageCount, err)
	}

	// 解散保留只读成员关系，通知当前成员而不发送失权通知。
	if _, err := conversationaction.NewDissolveGroupConversationAction(f.db, coordinator).Execute(ctx, f.member, group.ID); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, append(changed(f.owner, f.member), actorRead(f.member))...)
	// 以一次本人静音收尾，暴露解散后多余的通知。
	if _, err := conversationaction.NewUpdateConversationNotificationSettingsAction(f.db).Execute(ctx, f.owner, f.groupID, true); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, feed.notice(f.owner.User.ID, realtime.KindConversationStateChanged, f.groupID, loadConversationStateVersion(t, f.db, f.groupID, f.owner.User.ID)))
}

// testAgentRunNotifications 验证 AI 聊天运行开始、失败结果与崩溃恢复重入的会话变更通知，以及生成期间 AI 员工的输入状态。
func testAgentRunNotifications(t *testing.T, db *bun.DB, identity *servermodels.Identity, agentIdentityID string, tasks *servertask.Runtime) {
	ctx := context.Background()
	_, failing := createAgentLockChat(t, ctx, db, identity, agentIdentityID, tasks)
	_, recovering := createAgentLockChat(t, ctx, db, identity, agentIdentityID, tasks)
	feed := startRealtimeFeed(t, identity.Organization.ID)
	agentSubjectID := loadIdentitySubjectID(t, db, identity.Organization.ID, agentIdentityID)

	// 排队运行开始后模型失败，开始与失败结果各推进一次版本。
	var runningVersion int64
	failRuntime := testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, input agentruntime.InputFeed) (agentruntime.RunResult, error) {
		if _, err := input.Claim(ctx, 1); err != nil {
			return agentruntime.RunResult{}, err
		}
		runningVersion = loadConversationVersion(t, db, failing.ConversationID)
		return agentruntime.RunResult{}, errors.New("test model failure")
	}}
	if err := agentrunaction.NewExecuteAction(db, tasks, failRuntime, testAttachmentReader(db), nil).Execute(ctx, agentrunaction.RunInput{RunID: failing.ID}); err == nil {
		t.Fatal("model failure was not reported")
	}
	feed.expect(t,
		feed.notice(identity.User.ID, realtime.KindConversationChanged, failing.ConversationID, runningVersion),
		feed.userTyping(identity.User.ID, failing.ConversationID, agentSubjectID, true),
		feed.notice(identity.User.ID, realtime.KindConversationChanged, failing.ConversationID, loadConversationVersion(t, db, failing.ConversationID)),
		feed.userTyping(identity.User.ID, failing.ConversationID, agentSubjectID, false),
	)

	// 崩溃恢复重入运行中状态不推进版本，成功结果推进一次。
	if _, err := db.NewUpdate().Table("agent_runs").Set("status = ?", domain.AgentRunStatusRunning).Set("started_at = now()").Where("id = ?", recovering.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	before := loadConversationVersion(t, db, recovering.ConversationID)
	successRuntime := testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, input agentruntime.InputFeed) (agentruntime.RunResult, error) {
		claimed, err := input.Claim(ctx, 1)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		return agentruntime.RunResult{Content: "恢复后完成", EndSeq: claimed.EndSeq}, nil
	}}
	if err := agentrunaction.NewExecuteAction(db, tasks, successRuntime, testAttachmentReader(db), nil).Execute(ctx, agentrunaction.RunInput{RunID: recovering.ID}); err != nil {
		t.Fatal(err)
	}
	after := loadConversationVersion(t, db, recovering.ConversationID)
	if after != before+1 {
		t.Fatalf("recovered run version=%d want=%d", after, before+1)
	}
	feed.expect(t,
		feed.userTyping(identity.User.ID, recovering.ConversationID, agentSubjectID, true),
		feed.notice(identity.User.ID, realtime.KindConversationChanged, recovering.ConversationID, after),
		feed.userTyping(identity.User.ID, recovering.ConversationID, agentSubjectID, false),
	)
}

// TestRealtimeCustomerInboxNotifications 验证客户会话收发与服务周期变化通知企业客服共享受众和访客目录受众，客服已读只通知本人。
func TestRealtimeCustomerInboxNotifications(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	coordinator := newGroupAgentCoordinator(f.db)
	claim := conversationaction.NewClaimServiceSessionAction(f.db, coordinator, newTestTasks(f.db))
	closeSession := conversationaction.NewCloseServiceSessionAction(f.db, coordinator, newTestTasks(f.db))
	reopen := conversationaction.NewReopenServiceSessionAction(f.db)
	inbox := inboxaction.NewLoadInboxQuery(f.db)
	feed := startRealtimeFeed(t, f.owner.Organization.ID)
	visitorIdentityID := loadChannelIdentityID(t, f.db, f.conversationID)
	// changed 构造客户会话当前版本的共享受众与访客目录受众通知。
	changed := func() []receivedNotification {
		version := loadConversationVersion(t, f.db, f.conversationID)
		return []receivedNotification{feed.customerInbox(f.conversationID, version), feed.visitorDirectory(visitorIdentityID, f.conversationID, version)}
	}
	// listed 判断客户会话是否出现在指定客服的会话列表中。
	listed := func(identity *servermodels.Identity, input inboxaction.LoadInput) bool {
		page, _, err := inbox.Execute(ctx, identity, input)
		if err != nil {
			t.Fatal(err)
		}
		return slices.ContainsFunc(page.Conversations, func(row inboxaction.ConversationSummary) bool { return row.ID == f.conversationID })
	}
	// activity 读取客户会话的活动排序时间。
	activity := func() time.Time {
		var value time.Time
		if err := f.db.NewSelect().Table("conversations").Column("last_activity_at").Where("id = ?", f.conversationID).Scan(ctx, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}

	// 访客消息通知共享受众与访客目录受众，不逐客服扇出。
	if _, err := f.visitorMessage(ctx, "访客追问"); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed()...)

	// 领取后移出双方的待领取、进入负责人的等我回复并按负责人筛出，筛选迁移不影响双方按 ID 阅读。
	if _, err := claim.Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed()...)
	queued := inboxaction.LoadInput{Scope: domain.InboxScopePending, PendingKind: domain.InboxPendingKindQueue}
	ownedByOwner := inboxaction.LoadInput{Scope: domain.InboxScopeAll, AssigneeFilter: domain.InboxAssigneeFilterIdentity, AssigneeIdentityID: f.owner.OrganizationIdentity.ID}
	if listed(f.owner, queued) || listed(f.member, queued) || !listed(f.owner, inboxaction.LoadInput{Scope: domain.InboxScopePending, PendingKind: domain.InboxPendingKindReply}) || !listed(f.member, ownedByOwner) {
		t.Fatal("claimed conversation views did not converge")
	}
	for _, identity := range []*servermodels.Identity{f.owner, f.member} {
		results, err := inbox.ReadByIDs(ctx, identity, []string{f.conversationID}, nil)
		if err != nil || len(results) != 1 || results[0].Conversation == nil {
			t.Fatalf("claimed conversation unreadable results=%+v err=%v", results, err)
		}
	}
	// 负责人重复领取没有变化，不推进版本。
	version := loadConversationVersion(t, f.db, f.conversationID)
	if _, err := claim.Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatal(err)
	}
	if after := loadConversationVersion(t, f.db, f.conversationID); after != version {
		t.Fatalf("repeated claim version=%d want=%d", after, version)
	}

	// 负责人回复通知共享受众。
	if _, err := conversationaction.NewSendCustomerTextMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "客服回复"}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed()...)

	// 内部备注只通知企业客服共享受众，不登记访客目录受众。
	if _, err := conversationaction.NewSendCustomerTextMessageAction(f.db, nil).Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "内部备注：等仓库确认", Visibility: domain.MessageVisibilityInternalOnly}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, feed.customerInbox(f.conversationID, loadConversationVersion(t, f.db, f.conversationID)))

	// 转交通知共享受众；原负责人随后关闭被拒绝，不留通知。
	if _, err := conversationaction.NewTransferServiceSessionAction(f.db, coordinator, nil, newTestTasks(f.db)).Execute(ctx, f.owner, conversationaction.TransferServiceSessionInput{ConversationID: f.conversationID, TargetKind: domain.ServiceSessionTargetMember, IdentityID: f.member.OrganizationIdentity.ID}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed()...)
	if _, err := closeSession.Execute(ctx, f.owner, f.conversationID); err == nil {
		t.Fatal("former assignee closed the transferred session")
	}

	// 关闭与重开改变服务视图并通知，但不改变活动时间。
	lastActivity := activity()
	if _, err := closeSession.Execute(ctx, f.member, f.conversationID); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed()...)
	if !listed(f.member, inboxaction.LoadInput{Scope: domain.InboxScopeAll, AssigneeFilter: domain.InboxAssigneeFilterIdentity, AssigneeIdentityID: f.member.OrganizationIdentity.ID, ServiceStatus: domain.ServiceSessionStatusClosed}) {
		t.Fatal("closed conversation missing from closed view")
	}
	if _, err := reopen.Execute(ctx, f.member, f.conversationID); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed()...)
	if !activity().Equal(lastActivity) {
		t.Fatalf("service status changed activity from %s to %s", lastActivity, activity())
	}

	// 关闭后访客再发消息开启新周期并通知共享受众。
	if _, err := closeSession.Execute(ctx, f.member, f.conversationID); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed()...)
	next, err := f.visitorMessage(ctx, "新周期消息")
	if err != nil || !next.OpenedNewServiceSession {
		t.Fatalf("new service session result=%+v err=%v", next, err)
	}
	feed.expect(t, changed()...)

	// 客服已读只通知本人。
	if _, err := conversationaction.NewMarkConversationReadAction(f.db).Execute(ctx, f.member, f.conversationID, next.Message.ID, false); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, feed.notice(f.member.User.ID, realtime.KindConversationStateChanged, f.conversationID, loadConversationStateVersion(t, f.db, f.conversationID, f.member.User.ID)))
}

// TestRealtimeVisitorDirectoryNotifications 验证网站客户线程按所属渠道身份通知访客目录受众，不同访客身份互不接收，目录查询据此发现未知线程。
func TestRealtimeVisitorDirectoryNotifications(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	feed := startRealtimeFeed(t, f.owner.Organization.ID)
	directory := conversationaction.NewListWebsiteConversationsQuery(f.db)
	const visitor = "web-session:0123456789abcdef0123456789abcdef"
	const otherVisitor = "web-session:fedcba9876543210fedcba9876543210"
	visitorIdentityID := loadChannelIdentityID(t, f.db, f.conversationID)

	// 同一访客在另一标签页新建线程，共享受众与本人访客目录受众各收到一条通知。
	second, err := f.receive.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: f.channelID, ExternalID: visitor, ClientMessageID: uuid.NewV7().String(), Body: "第二个线程",
	})
	if err != nil || !second.CreatedConversation {
		t.Fatalf("second thread=%+v err=%v", second, err)
	}
	secondVersion := loadConversationVersion(t, f.db, second.Conversation.ID)
	feed.expect(t, feed.customerInbox(second.Conversation.ID, secondVersion), feed.visitorDirectory(visitorIdentityID, second.Conversation.ID, secondVersion))

	// 目录查询按渠道身份列出两个线程，另一标签页据此发现未知线程。
	threads, err := directory.Execute(ctx, f.channelID, visitor)
	if err != nil {
		t.Fatal(err)
	}
	listed := make([]string, 0, len(threads))
	for _, thread := range threads {
		listed = append(listed, thread.ID)
	}
	slices.Sort(listed)
	want := []string{f.conversationID, second.Conversation.ID}
	slices.Sort(want)
	if !slices.Equal(listed, want) {
		t.Fatalf("访客目录 got=%v want=%v", listed, want)
	}

	// 另一访客身份的线程只通知其自身受众，且不出现在前一访客的目录中。
	other, err := f.receive.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: f.channelID, ExternalID: otherVisitor, ClientMessageID: uuid.NewV7().String(), Body: "另一访客的线程",
	})
	if err != nil {
		t.Fatal(err)
	}
	otherIdentityID := loadChannelIdentityID(t, f.db, other.Conversation.ID)
	if otherIdentityID == visitorIdentityID {
		t.Fatalf("两个访客共用渠道身份 %s", otherIdentityID)
	}
	otherVersion := loadConversationVersion(t, f.db, other.Conversation.ID)
	feed.expect(t, feed.customerInbox(other.Conversation.ID, otherVersion), feed.visitorDirectory(otherIdentityID, other.Conversation.ID, otherVersion))
	threads, err = directory.Execute(ctx, f.channelID, visitor)
	if err != nil {
		t.Fatal(err)
	}
	listed = listed[:0]
	for _, thread := range threads {
		listed = append(listed, thread.ID)
	}
	slices.Sort(listed)
	if !slices.Equal(listed, want) {
		t.Fatalf("跨访客目录隔离 got=%v want=%v", listed, want)
	}
}

// TestRealtimeCustomerDeliveryNotifications 验证投递状态变化、渠道启停与更换机器人推进客户会话版本并通知共享受众，无变化的写入不推进。
func TestRealtimeCustomerDeliveryNotifications(t *testing.T) {
	f := newCustomerDeliveryFixture(t)
	ctx := context.Background()
	feed := startRealtimeFeed(t, f.owner.Organization.ID)
	// changed 构造比客户会话当前版本低 offset 的共享受众通知。
	changed := func(offset int64) receivedNotification {
		return feed.customerInbox(f.conversationID, loadConversationVersion(t, f.db, f.conversationID)-offset)
	}
	// unchanged 执行一次投递并断言会话版本不变。
	unchanged := func(id string) {
		version := loadConversationVersion(t, f.db, f.conversationID)
		f.execute(t, id)
		if after := loadConversationVersion(t, f.db, f.conversationID); after != version {
			t.Fatalf("no-op delivery version=%d want=%d", after, version)
		}
	}

	// 回复与入队同事务通知一次。
	first := f.send(t, "第一条", uuid.NewV7().String())
	feed.expect(t, changed(0))
	second := f.send(t, "第二条", uuid.NewV7().String())
	feed.expect(t, changed(0))
	// 队头未完成时第二条认领不写入。
	unchanged(second.ID)

	// 认领与发送结果各推进一次版本。
	f.sender.err = &telegramintegration.SendError{Code: "recipient_unavailable"}
	if got := f.execute(t, first.ID); got.Status != domain.CustomerDeliveryFailed {
		t.Fatalf("delivery status=%s", got.Status)
	}
	feed.expect(t, changed(1), changed(0))

	// 人工重试通知共享受众。
	f.sender.err = nil
	if err := deliveryaction.NewManager(f.db, nil).Resolve(ctx, f.owner, f.conversationID, first.ID, domain.CustomerDeliveryRetry, false); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed(0))

	// 停用渠道推进有待发送投递的会话版本，重复停用不推进。
	api := &telegramBotAPIFake{bot: telegramintegration.Bot{ID: 456, IsBot: true, FirstName: "新机器人", Username: "new_delivery_bot"}}
	runner := connectiontest.NewRunner(time.Second)
	updateStatus := channelaction.NewUpdateTelegramChannelStatusAction(f.db, runner, api)
	if _, err := updateStatus.Execute(ctx, f.owner, f.channelID, false); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed(0))
	version := loadConversationVersion(t, f.db, f.conversationID)
	if _, err := updateStatus.Execute(ctx, f.owner, f.channelID, false); err != nil {
		t.Fatal(err)
	}
	if after := loadConversationVersion(t, f.db, f.conversationID); after != version {
		t.Fatalf("repeated disable version=%d want=%d", after, version)
	}
	// 停用期间认领队头不写入投递，不推进版本。
	unchanged(second.ID)
	// 重新启用渠道推进版本。
	if _, err := f.db.ExecContext(ctx, "UPDATE telegram_channel_settings SET webhook_base_url = 'https://example.com' WHERE channel_id = ?", f.channelID); err != nil {
		t.Fatal(err)
	}
	if _, err := updateStatus.Execute(ctx, f.owner, f.channelID, true); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed(0))

	// 更换机器人推进渠道内客户会话版本并通知一次。
	if _, err := channelaction.NewSaveTelegramConnectionAction(f.db, runner, api).Execute(ctx, f.owner, f.channelID, channelaction.TelegramChannelConnectionInput{BotToken: "456:new_token", WebhookBaseURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	feed.expect(t, changed(0))
	if got := f.load(t, first.ID); got.Status != domain.CustomerDeliveryFailed || got.LastError != "bot_changed" {
		t.Fatalf("bot changed delivery=%+v", got)
	}
}

// testCustomerAgentRunNotifications 验证客服 Agent 运行开始与最终回复通知企业客服共享受众和访客目录受众，生成期间两类受众都收到 AI 员工正在输入。
func testCustomerAgentRunNotifications(t *testing.T, db *bun.DB, identity *servermodels.Identity, agentIdentityID string, tasks *servertask.Runtime) {
	ctx := context.Background()
	first, _, run := createCustomerLockRun(t, ctx, db, identity, agentIdentityID, tasks)
	feed := startRealtimeFeed(t, identity.Organization.ID)
	visitorIdentityID := loadChannelIdentityID(t, db, first.Conversation.ID)
	var runningVersion int64
	runtime := testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, input agentruntime.InputFeed) (agentruntime.RunResult, error) {
		claimed, err := input.Claim(ctx, 1)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		runningVersion = loadConversationVersion(t, db, first.Conversation.ID)
		return agentruntime.RunResult{Content: "AI 最终回复", EndSeq: claimed.EndSeq}, nil
	}}
	if err := agentrunaction.NewExecuteAction(db, tasks, runtime, testAttachmentReader(db), nil).Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	finalVersion := loadConversationVersion(t, db, first.Conversation.ID)
	// 聊天主体在运行开始时建立，运行结束后读取。
	agentSubjectID := loadIdentitySubjectID(t, db, identity.Organization.ID, agentIdentityID)
	feed.expect(t,
		feed.customerInbox(first.Conversation.ID, runningVersion),
		feed.visitorDirectory(visitorIdentityID, first.Conversation.ID, runningVersion),
		// AI 生成期间客服与访客看到正在输入，运行结束后收到停止。
		feed.customerInboxTyping(first.Conversation.ID, agentSubjectID, true),
		feed.visitorTyping(visitorIdentityID, first.Conversation.ID, true),
		feed.customerInbox(first.Conversation.ID, finalVersion),
		feed.visitorDirectory(visitorIdentityID, first.Conversation.ID, finalVersion),
		feed.customerInboxTyping(first.Conversation.ID, agentSubjectID, false),
		feed.visitorTyping(visitorIdentityID, first.Conversation.ID, false),
	)
}
