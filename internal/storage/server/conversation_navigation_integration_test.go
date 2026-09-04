//go:build server

package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type navigationFixture struct {
	db                 *bun.DB
	owner, member      *servermodels.Identity
	groupID, subjectID string
}

// newNavigationFixture 安装独立测试企业并建立两人群聊。
func newNavigationFixture(t *testing.T) navigationFixture {
	t.Helper()
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db := store.DB()
	installed, err := installationaction.NewInstallWorkspaceAction(db).Execute(ctx, installationaction.InstallWorkspaceInput{
		AccessHost: uuid.NewV7().String() + ".navigation.test", OrganizationName: "导航测试", DisplayName: "群主", Email: "owner@navigation.test", Password: "password123", Locale: domain.LocaleEnglishUnitedStates, TimeZone: "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	owner := installed.Identity
	_, err = useraction.NewCreateUserAction(db).Execute(ctx, owner, useraction.CreateInput{DisplayName: "成员", Email: "member@navigation.test", Password: "password123", RoleID: owner.OrganizationIdentity.RoleID})
	if err != nil {
		t.Fatal(err)
	}
	login, err := authaction.NewLoginAction(db).Execute(ctx, authaction.LoginInput{OrganizationID: owner.Organization.ID, Email: "member@navigation.test", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	group, err := conversationaction.NewCreateGroupConversationAction(db).Execute(ctx, owner, conversationaction.GroupConversationInput{Title: "导航测试群", MemberIdentityIDs: []string{login.Identity.OrganizationIdentity.ID}})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := conversationaction.NewGetGroupConversationQuery(db).Execute(ctx, owner, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture := navigationFixture{db: db, owner: owner, member: login.Identity, groupID: group.ID}
	for _, participant := range detail.Participants {
		if participant.IdentityID == fixture.member.OrganizationIdentity.ID {
			fixture.subjectID = participant.ChatSubjectID
		}
	}
	return fixture
}

// send 通过真实发送命令写入测试消息。
func (f navigationFixture) send(t *testing.T, identity *servermodels.Identity, body string, all bool, subjects ...string) conversationaction.ConversationMessage {
	t.Helper()
	message, err := conversationaction.NewSendGroupTextMessageAction(f.db).Execute(context.Background(), identity, conversationaction.GroupTextMessageInput{ConversationID: f.groupID, ClientMessageID: uuid.NewV7().String(), Body: body, MentionAll: all, MentionSubjectIDs: subjects})
	if err != nil {
		t.Fatal(err)
	}
	return message
}

// state 读取个人水位，保留被删除消息的指针。
func (f navigationFixture) state(t *testing.T) servermodels.ConversationUserState {
	t.Helper()
	var state servermodels.ConversationUserState
	if err := f.db.NewSelect().Model(&state).Where("cus.organization_id = ? AND cus.conversation_id = ? AND cus.user_id = ?", f.owner.Organization.ID, f.groupID, f.member.User.ID).Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	return state
}

// TestGroupMentionNavigation 验证独立水位、连续确认、删除、跨企业边界及重新入群。
func TestGroupMentionNavigation(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	pending := conversationaction.NewListPendingConversationMentionsQuery(f.db)
	navigation := conversationaction.NewGetConversationNavigationStateQuery(f.db)
	review := conversationaction.NewMarkConversationMentionReviewedAction(f.db)
	read := conversationaction.NewMarkConversationReadAction(f.db)
	first := f.send(t, f.owner, "第一条提及", false, f.subjectID)
	second := f.send(t, f.owner, "两种提及合并一次", true, f.subjectID)
	f.send(t, f.member, "自己的所有人消息", true)
	last := f.send(t, f.owner, "普通消息", false)
	queue, err := pending.Execute(ctx, f.member, f.groupID)
	if err != nil || !slices.Equal(queue.MessageIDs, []string{first.ID, second.ID}) || queue.LastTargetSequence == nil || *queue.LastTargetSequence != *second.ConversationSequence {
		t.Fatalf("queue=%+v err=%v", queue, err)
	}
	if _, err := read.Execute(ctx, f.member, f.groupID, last.ID); err != nil {
		t.Fatal(err)
	}
	status, err := navigation.Execute(ctx, f.member, f.groupID)
	if err != nil || status.PendingMentionCount != 2 || status.ReviewedThroughSequence != 0 || status.LatestMessageID == nil || *status.LatestMessageID != last.ID {
		t.Fatalf("navigation=%+v err=%v", status, err)
	}
	if _, err := review.Execute(ctx, f.member, f.groupID, second.ID); !errors.Is(err, conversationaction.ErrMentionProgressChanged) {
		t.Fatalf("skipped first mention: %v", err)
	}
	if _, err := review.Execute(ctx, f.member, f.groupID, last.ID); !errors.Is(err, conversationaction.ErrMentionTargetInvalid) {
		t.Fatalf("accepted ordinary message: %v", err)
	}
	result, err := review.Execute(ctx, f.member, f.groupID, first.ID)
	if err != nil || result.Outcome != "reviewed" {
		t.Fatalf("review=%+v err=%v", result, err)
	}
	result, err = review.Execute(ctx, f.member, f.groupID, first.ID)
	if err != nil || result.Outcome != "alreadyReviewed" {
		t.Fatalf("repeated review=%+v err=%v", result, err)
	}
	state := f.state(t)
	if state.LastReadMessageID == nil || *state.LastReadMessageID != last.ID || state.LastReviewedMentionMessageID == nil || *state.LastReviewedMentionMessageID != first.ID {
		t.Fatalf("watermarks coupled: %+v", state)
	}
	if _, err := f.db.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id IN (?)", bun.In([]string{first.ID, second.ID})).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	result, err = review.Execute(ctx, f.member, f.groupID, second.ID)
	if err != nil || result.Outcome != "unavailable" || result.ReviewedThroughSequence != *first.ConversationSequence {
		t.Fatalf("deleted review=%+v err=%v", result, err)
	}
	third := f.send(t, f.owner, "删除后继续", false, f.subjectID)
	result, err = review.Execute(ctx, f.member, f.groupID, third.ID)
	if err != nil || result.Outcome != "reviewed" {
		t.Fatalf("review after deletion=%+v err=%v", result, err)
	}
	if _, err := read.Execute(ctx, f.member, f.groupID, first.ID); err != nil {
		t.Fatal(err)
	}
	if state = f.state(t); *state.LastReadMessageID != last.ID {
		t.Fatalf("read regressed: %+v", state)
	}
	if _, err := conversationaction.NewUpdateConversationNotificationSettingsAction(f.db).Execute(ctx, f.member, f.groupID, true); err != nil {
		t.Fatal(err)
	}
	f.send(t, f.owner, "离群前未查看", true)
	if _, err := conversationaction.NewRemoveGroupConversationMemberAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationMemberInput{ConversationID: f.groupID, MemberIdentityID: f.member.OrganizationIdentity.ID}); err != nil {
		t.Fatal(err)
	}
	for _, readQuery := range []func() error{
		func() error { _, err := pending.Execute(ctx, f.member, f.groupID); return err },
		func() error { _, err := navigation.Execute(ctx, f.member, f.groupID); return err },
		func() error { _, err := review.Execute(ctx, f.member, f.groupID, third.ID); return err },
	} {
		if err := readQuery(); !errors.Is(err, conversationaction.ErrConversationNotFound) {
			t.Fatalf("removed member access: %v", err)
		}
	}
	if _, err := conversationaction.NewAddGroupConversationMembersAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationMembersInput{ConversationID: f.groupID, MemberIdentityIDs: []string{f.member.OrganizationIdentity.ID}}); err != nil {
		t.Fatal(err)
	}
	state = f.state(t)
	if !state.Muted || state.LastReadMessageID == nil || state.LastReviewedMentionMessageID == nil || *state.LastReadMessageID != *state.LastReviewedMentionMessageID {
		t.Fatalf("rejoin state=%+v", state)
	}
	queue, err = pending.Execute(ctx, f.member, f.groupID)
	if err != nil || len(queue.MessageIDs) != 0 {
		t.Fatalf("rejoin queue=%+v err=%v", queue, err)
	}
	archivedTarget := f.send(t, f.owner, "归档仍可查看", true)
	if _, err := f.db.NewUpdate().Model((*servermodels.Conversation)(nil)).Set("status = ?", domain.ConversationStatusArchived).Where("id = ?", f.groupID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	result, err = review.Execute(ctx, f.member, f.groupID, archivedTarget.ID)
	if err != nil || result.Outcome != "reviewed" {
		t.Fatalf("archived review=%+v err=%v", result, err)
	}
	// 另一企业的有效身份不能读取或确认当前企业的群聊。
	other := newNavigationFixture(t)
	if _, err := pending.Execute(ctx, other.member, f.groupID); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("cross-tenant queue: %v", err)
	}
	if _, err := review.Execute(ctx, other.member, f.groupID, archivedTarget.ID); !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("cross-tenant review: %v", err)
	}
}

// TestGroupMessageContextAndOrder 验证双向窗口、删除引用和并发写入的稳定顺序。
func TestGroupMessageContextAndOrder(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	send := conversationaction.NewSendGroupTextMessageAction(f.db)
	messages := make([]conversationaction.ConversationMessage, 70)
	for index := range messages {
		messages[index] = f.send(t, f.owner, fmt.Sprintf("消息 %d", index), index == 30)
	}
	history := conversationaction.NewListConversationMessagesQuery(f.db)
	window, err := history.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID, AroundMessageID: messages[35].ID})
	if err != nil || len(window.Messages) != 51 || !window.HasEarlier || !window.HasLater || window.Before == nil || window.After == nil || window.Messages[25].ID != messages[35].ID {
		t.Fatalf("context=%+v err=%v", window, err)
	}
	earlier, err := history.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID, Before: window.Before})
	if err != nil || earlier.HasEarlier || !earlier.HasLater || len(earlier.Messages) != 10 {
		t.Fatalf("earlier=%+v err=%v", earlier, err)
	}
	later, err := history.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID, After: window.After})
	if err != nil || later.HasLater || !later.HasEarlier || len(later.Messages) != 9 {
		t.Fatalf("later=%+v err=%v", later, err)
	}
	reply, err := send.Execute(ctx, f.owner, conversationaction.GroupTextMessageInput{ConversationID: f.groupID, ClientMessageID: uuid.NewV7().String(), Body: "引用", ReplyToMessageID: messages[35].ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id = ?", messages[35].ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID, AroundMessageID: messages[35].ID}); !errors.Is(err, conversationaction.ErrMessageUnavailable) {
		t.Fatalf("deleted context: %v", err)
	}
	window, err = history.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID, AroundMessageID: reply.ID})
	if err != nil {
		t.Fatal(err)
	}
	reference := window.Messages[len(window.Messages)-1].ReplyTo
	if reference == nil || !reference.Deleted || reference.Body != "" || reference.ID != messages[35].ID {
		t.Fatalf("deleted reference=%+v", reference)
	}
	// 相同幂等键并发重试只占用一个序号，两个发送者共享同一递增序列。
	const writers = 12
	var wait sync.WaitGroup
	errs := make(chan error, writers)
	input := conversationaction.GroupTextMessageInput{ConversationID: f.groupID, ClientMessageID: uuid.NewV7().String(), Body: "幂等重试"}
	for index := range writers {
		wait.Go(func() {
			identity := f.owner
			current := input
			if index%2 == 1 {
				identity = f.member
				current.ClientMessageID = uuid.NewV7().String()
			}
			_, err := send.Execute(ctx, identity, current)
			errs <- err
		})
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var sequences []int64
	if err := f.db.NewSelect().Model((*servermodels.Message)(nil)).Column("conversation_sequence").Where("conversation_id = ?", f.groupID).Order("conversation_sequence ASC").Scan(ctx, &sequences); err != nil {
		t.Fatal(err)
	}
	if len(sequences) != 78 {
		t.Fatalf("message count=%d want78", len(sequences))
	}
	for index, sequence := range sequences {
		if sequence != int64(index+1) {
			t.Fatalf("sequence[%d]=%d", index, sequence)
		}
	}
	// 改变来源时间不会改变群聊最新页、游标和普通已读的顺序。
	if _, err := f.db.NewUpdate().Model((*servermodels.Message)(nil)).Set("originated_at = ?", time.Now().Add(24*time.Hour)).Where("id = ?", messages[0].ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	latest, err := history.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.groupID})
	if err != nil || *latest.Messages[len(latest.Messages)-1].ConversationSequence != 78 {
		t.Fatalf("latest order err=%v", err)
	}
}

type navigationWriteBarrier struct {
	body             string
	entered, release chan struct{}
	once             sync.Once
}

// BeforeQuery 保留原查询上下文。
func (b *navigationWriteBarrier) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

// AfterQuery 在消息插入后、事务提交前建立可控屏障。
func (b *navigationWriteBarrier) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	if event.Model == nil {
		return
	}
	message, ok := event.Model.Value().(*servermodels.Message)
	if !ok || message.Body != b.body || event.Operation() != "INSERT" || event.Err != nil {
		return
	}
	b.once.Do(func() {
		close(b.entered)
		select {
		case <-b.release:
		case <-ctx.Done():
		}
	})
}

// TestGroupSequenceCommitBarrier 验证未提交消息阻止后续序号绕过，并发确认保持连续幂等。
func TestGroupSequenceCommitBarrier(t *testing.T) {
	f := newNavigationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	barrier := &navigationWriteBarrier{body: "屏障内未提交提及", entered: make(chan struct{}), release: make(chan struct{})}
	// 查询钩子只在本测试专用连接上阻塞指定消息。
	f.db.AddQueryHook(barrier)
	var release sync.Once
	defer release.Do(func() { close(barrier.release) })
	type sent struct {
		message conversationaction.ConversationMessage
		err     error
	}
	firstDone, secondDone := make(chan sent, 1), make(chan sent, 1)
	send := conversationaction.NewSendGroupTextMessageAction(f.db)
	go func() {
		message, err := send.Execute(ctx, f.owner, conversationaction.GroupTextMessageInput{ConversationID: f.groupID, ClientMessageID: uuid.NewV7().String(), Body: barrier.body, MentionAll: true})
		firstDone <- sent{message, err}
	}()
	select {
	case <-barrier.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	go func() {
		message, err := send.Execute(ctx, f.member, conversationaction.GroupTextMessageInput{ConversationID: f.groupID, ClientMessageID: uuid.NewV7().String(), Body: "后续提及", MentionAll: true})
		secondDone <- sent{message, err}
	}()
	waitForNavigationLock(t, ctx, f.db, f.groupID)
	navigation := conversationaction.NewGetConversationNavigationStateQuery(f.db)
	state, err := navigation.Execute(ctx, f.member, f.groupID)
	if err != nil || state.PendingMentionCount != 0 || state.LatestSequence != 0 {
		t.Fatalf("uncommitted message visible: %+v %v", state, err)
	}
	release.Do(func() { close(barrier.release) })
	first, second := <-firstDone, <-secondDone
	if first.err != nil || second.err != nil || *first.message.ConversationSequence != 1 || *second.message.ConversationSequence != 2 {
		t.Fatalf("commit order: %+v %+v", first, second)
	}
	// 两端同时确认同一条提醒，恰好一次推进且两次都成功。
	review := conversationaction.NewMarkConversationMentionReviewedAction(f.db)
	results := make(chan conversationaction.ConversationMentionReview, 2)
	errorsOut := make(chan error, 2)
	for range 2 {
		go func() {
			result, err := review.Execute(ctx, f.member, f.groupID, first.message.ID)
			results <- result
			errorsOut <- err
		}()
	}
	outcomes := []string{(<-results).Outcome, (<-results).Outcome}
	if err := <-errorsOut; err != nil {
		t.Fatal(err)
	}
	if err := <-errorsOut; err != nil {
		t.Fatal(err)
	}
	slices.Sort(outcomes)
	if !slices.Equal(outcomes, []string{"alreadyReviewed", "reviewed"}) {
		t.Fatalf("concurrent review outcomes=%v", outcomes)
	}
}

// waitForNavigationLock 等待指定会话确实发生数据库锁竞争。
func waitForNavigationLock(t *testing.T, ctx context.Context, db *bun.DB, conversationID string) {
	t.Helper()
	for {
		var waiting bool
		err := db.NewSelect().TableExpr("pg_stat_activity").ColumnExpr("EXISTS (SELECT 1 FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query LIKE ?)", "%"+conversationID+"%").Limit(1).Scan(ctx, &waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// TestRemovedMemberCannotSendAfterWaiting 验证发送等待群锁后重新检查已提交的移除结果。
func TestRemovedMemberCannotSendAfterWaiting(t *testing.T) {
	f := newNavigationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	barrier := &navigationWriteBarrier{entered: make(chan struct{}), release: make(chan struct{})}
	f.db.AddQueryHook(barrier)
	var release sync.Once
	defer release.Do(func() { close(barrier.release) })
	removed, sent := make(chan error, 1), make(chan error, 1)
	go func() {
		_, err := conversationaction.NewRemoveGroupConversationMemberAction(f.db).Execute(ctx, f.owner, conversationaction.GroupConversationMemberInput{ConversationID: f.groupID, MemberIdentityID: f.member.OrganizationIdentity.ID})
		removed <- err
	}()
	select {
	case <-barrier.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	go func() {
		_, err := conversationaction.NewSendGroupTextMessageAction(f.db).Execute(ctx, f.member, conversationaction.GroupTextMessageInput{ConversationID: f.groupID, ClientMessageID: uuid.NewV7().String(), Body: "已被移除成员不能发送"})
		sent <- err
	}()
	waitForNavigationLock(t, ctx, f.db, f.groupID)
	release.Do(func() { close(barrier.release) })
	if err := <-removed; err != nil {
		t.Fatal(err)
	}
	if err := <-sent; !errors.Is(err, conversationaction.ErrConversationNotFound) {
		t.Fatalf("removed member send=%v", err)
	}
	var count int
	if err := f.db.NewSelect().TableExpr("messages").ColumnExpr("count(*)").Where("conversation_id = ?", f.groupID).Scan(ctx, &count); err != nil || count != 1 {
		t.Fatalf("message count=%d err=%v", count, err)
	}
}
