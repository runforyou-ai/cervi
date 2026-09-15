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
	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/servertest"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// receivedNotification 表示订阅端收到的一条实时通知。
type receivedNotification struct {
	Subject        string
	Kind           string
	ConversationID string
	Version        string
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

// removed 构造发往指定用户受众的会话失权通知。
func (f *realtimeFeed) removed(userID, conversationID string) receivedNotification {
	return receivedNotification{
		Subject: realtime.Subject(f.namespace, f.organizationID, realtime.AudienceUser, userID),
		Kind:    string(realtime.KindConversationRemoved), ConversationID: conversationID,
	}
}

// expect 读取与期望数量相同的通知，并与期望集合按任意顺序比较。
func (f *realtimeFeed) expect(t *testing.T, want ...receivedNotification) {
	t.Helper()
	got := make([]receivedNotification, 0, len(want))
	for range want {
		message, err := f.subscription.NextMsg(5 * time.Second)
		if err != nil {
			t.Fatalf("等待实时通知: %v，已收到 %+v", err, got)
		}
		var fields map[string]string
		if err := json.Unmarshal(message.Data, &fields); err != nil {
			t.Fatalf("解析实时通知 %s: %v", message.Data, err)
		}
		// 载荷只允许种类、会话 ID 与版本。
		for key := range fields {
			if key != "kind" && key != "conversationId" && key != "version" {
				t.Fatalf("实时通知含业务字段: %s", message.Data)
			}
		}
		got = append(got, receivedNotification{Subject: message.Subject, Kind: fields["kind"], ConversationID: fields["conversationId"], Version: fields["version"]})
	}
	compare := func(a, b receivedNotification) int {
		return strings.Compare(a.Subject+a.Kind+a.ConversationID+a.Version, b.Subject+b.Kind+b.ConversationID+b.Version)
	}
	slices.SortFunc(got, compare)
	slices.SortFunc(want, compare)
	if !slices.Equal(got, want) {
		t.Fatalf("实时通知不符\ngot=%+v\nwant=%+v", got, want)
	}
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
	workStatus := useraction.NewUpdateWorkStatusAction(f.db)
	preferences := useraction.NewUpdatePreferencesAction(f.db)
	updateUser := useraction.NewUpdateUserAction(f.db)
	for _, step := range []struct {
		name   string
		change func() error
	}{
		{"工作状态", func() error {
			_, err := workStatus.Execute(ctx, f.member, useraction.WorkStatusInput{WorkStatus: domain.WorkStatusAway})
			return err
		}},
		{"账户偏好", func() error {
			_, err := preferences.Execute(ctx, f.member, useraction.PreferencesInput{Locale: domain.Locale(f.member.User.Locale), TimeZone: "Asia/Shanghai", MessageNotificationsEnabled: f.member.User.MessageNotificationsEnabled, WorkspaceTabsEnabled: f.member.User.WorkspaceTabsEnabled})
			return err
		}},
		{"管理员修改邮箱", func() error {
			_, err := updateUser.Execute(ctx, f.owner, f.member.User.ID, useraction.UpdateInput{DisplayName: "成员", Email: "renamed@navigation.test", RoleID: f.member.OrganizationIdentity.RoleID})
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
	if err := conversationaction.NewLeaveGroupConversationAction(f.db).Execute(ctx, third, group.ID); err != nil {
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

// testAgentRunNotifications 验证 AI 聊天运行开始、失败结果与崩溃恢复重入的会话变更通知。
func testAgentRunNotifications(t *testing.T, db *bun.DB, identity *servermodels.Identity, agentIdentityID string, tasks *servertask.Runtime) {
	ctx := context.Background()
	_, failing := createAgentLockChat(t, ctx, db, identity, agentIdentityID, tasks)
	_, recovering := createAgentLockChat(t, ctx, db, identity, agentIdentityID, tasks)
	feed := startRealtimeFeed(t, identity.Organization.ID)

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
		feed.notice(identity.User.ID, realtime.KindConversationChanged, failing.ConversationID, loadConversationVersion(t, db, failing.ConversationID)),
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
	feed.expect(t, feed.notice(identity.User.ID, realtime.KindConversationChanged, recovering.ConversationID, after))
}
