//go:build server

package integrationtest

import (
	"context"
	"errors"
	"testing"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
)

// TestCustomerNoteMentions 验证内部备注提醒建立协作者、@我的视图、提及计数和周期关闭后的移出。
func TestCustomerNoteMentions(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	send := conversationaction.NewSendCustomerTextMessageAction(f.db, nil)
	load := inboxaction.NewLoadInboxQuery(f.db)
	memberID := f.member.OrganizationIdentity.ID
	// customerRow 读取指定身份在客户收件箱某个视图中的目标会话摘要。
	customerRow := func(identity *servermodels.Identity, view domain.CustomerInboxView) (*inboxaction.ConversationSummary, inboxaction.UnreadCounts) {
		t.Helper()
		page, counts, err := load.Execute(ctx, identity, inboxaction.LoadInput{Scope: domain.InboxScopeCustomer, CustomerView: view, ServiceStatus: domain.ServiceSessionStatusOpen})
		if err != nil {
			t.Fatal(err)
		}
		for index := range page.Conversations {
			if page.Conversations[index].ID == f.conversationID {
				return &page.Conversations[index], counts
			}
		}
		return nil, counts
	}

	if row, counts := customerRow(f.member, domain.CustomerInboxViewMentioned); row != nil || counts.CustomerMentioned != 0 {
		t.Fatalf("mentioned view before note = %+v counts=%+v", row, counts)
	}
	noteInput := conversationaction.CustomerTextMessageInput{
		ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(),
		Body: "@成员 帮忙看下物流", Visibility: domain.MessageVisibilityInternalOnly, MentionIdentityIDs: []string{memberID},
	}
	note, err := send.Execute(ctx, f.owner, noteInput)
	if err != nil || len(note.Mentions) != 1 || note.Mentions[0].SourceID != memberID {
		t.Fatalf("note=%+v err=%v", note, err)
	}

	// 被提醒成员写入参与者关系，负责人保持不变。
	var participants int
	participants, err = f.db.NewSelect().TableExpr("conversation_participants AS cp").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id").
		Where("cp.conversation_id = ? AND cp.left_at IS NULL AND cs.source_id = ?", f.conversationID, memberID).Count(ctx)
	if err != nil || participants != 1 {
		t.Fatalf("collaborator participants=%d err=%v", participants, err)
	}
	session := &servermodels.ServiceSession{}
	if err := f.db.NewSelect().Model(session).Where("ss.conversation_id = ?", f.conversationID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if session.AssigneeIdentityID != nil {
		t.Fatalf("mention changed assignee: %+v", session)
	}

	row, counts := customerRow(f.member, domain.CustomerInboxViewMentioned)
	if row == nil || row.MentionedUnreadCount != 1 || counts.CustomerMentioned != 1 {
		t.Fatalf("mentioned view row=%+v counts=%+v", row, counts)
	}
	if row, _ := customerRow(f.owner, domain.CustomerInboxViewMentioned); row != nil {
		t.Fatalf("sender sees own mention in mentioned view: %+v", row)
	}
	if row, _ := customerRow(f.owner, domain.CustomerInboxViewQueue); row == nil || row.Customer.UnansweredMentionCount != 1 {
		t.Fatalf("unanswered mentions = %+v", row)
	}

	history, err := conversationaction.NewListConversationMessagesQuery(f.db).Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID})
	if err != nil {
		t.Fatal(err)
	}
	if last := history.Messages[len(history.Messages)-1]; last.ID != note.ID || len(last.Mentions) != 1 || last.Mentions[0].SourceID != memberID {
		t.Fatalf("timeline mentions = %+v", last)
	}

	t.Run("提及导航覆盖客户会话", func(t *testing.T) {
		state, err := conversationaction.NewGetConversationNavigationStateQuery(f.db).Execute(ctx, f.member, f.conversationID)
		if err != nil || state.PendingMentionCount != 1 {
			t.Fatalf("navigation state=%+v err=%v", state, err)
		}
		pending, err := conversationaction.NewListPendingConversationMentionsQuery(f.db).Execute(ctx, f.member, f.conversationID)
		if err != nil || len(pending.MessageIDs) != 1 || pending.MessageIDs[0] != note.ID {
			t.Fatalf("pending mentions=%+v err=%v", pending, err)
		}
		// 未被提醒的成员没有待查看提及，也不能确认他人的提醒。
		ownerState, err := conversationaction.NewGetConversationNavigationStateQuery(f.db).Execute(ctx, f.owner, f.conversationID)
		if err != nil || ownerState.PendingMentionCount != 0 {
			t.Fatalf("owner navigation state=%+v err=%v", ownerState, err)
		}
		if _, err := conversationaction.NewMarkConversationMentionReviewedAction(f.db).Execute(ctx, f.owner, f.conversationID, note.ID); !errors.Is(err, conversationaction.ErrMentionTargetInvalid) {
			t.Fatalf("owner review = %v", err)
		}
		review, err := conversationaction.NewMarkConversationMentionReviewedAction(f.db).Execute(ctx, f.member, f.conversationID, note.ID)
		if err != nil || review.Outcome != "reviewed" {
			t.Fatalf("review=%+v err=%v", review, err)
		}
		state, err = conversationaction.NewGetConversationNavigationStateQuery(f.db).Execute(ctx, f.member, f.conversationID)
		if err != nil || state.PendingMentionCount != 0 {
			t.Fatalf("navigation state after review=%+v err=%v", state, err)
		}
	})

	t.Run("阅读后不再计数但仍留在视图中", func(t *testing.T) {
		if _, err := conversationaction.NewMarkConversationReadAction(f.db).Execute(ctx, f.member, f.conversationID, note.ID, false); err != nil {
			t.Fatal(err)
		}
		row, counts := customerRow(f.member, domain.CustomerInboxViewMentioned)
		if row == nil || row.MentionedUnreadCount != 0 || counts.CustomerMentioned != 0 {
			t.Fatalf("mentioned view after read row=%+v counts=%+v", row, counts)
		}
	})

	t.Run("被提醒成员发言后提醒视为已回复", func(t *testing.T) {
		if _, err := send.Execute(ctx, f.member, conversationaction.CustomerTextMessageInput{
			ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(),
			Body: "物流已催", Visibility: domain.MessageVisibilityInternalOnly,
		}); err != nil {
			t.Fatal(err)
		}
		if row, _ := customerRow(f.owner, domain.CustomerInboxViewQueue); row == nil || row.Customer.UnansweredMentionCount != 0 {
			t.Fatalf("unanswered mentions after reply = %+v", row)
		}
	})

	t.Run("提醒目标校验", func(t *testing.T) {
		cases := []struct {
			name  string
			input conversationaction.CustomerTextMessageInput
		}{
			{"对客消息不能提醒", conversationaction.CustomerTextMessageInput{Body: "您好", MentionIdentityIDs: []string{memberID}}},
			{"不能重复提醒", conversationaction.CustomerTextMessageInput{Body: "看下", Visibility: domain.MessageVisibilityInternalOnly, MentionIdentityIDs: []string{memberID, memberID}}},
		}
		for _, test := range cases {
			test.input.ConversationID, test.input.ClientMessageID = f.conversationID, uuid.NewV7().String()
			var validation *conversationaction.ValidationError
			if _, err := send.Execute(ctx, f.owner, test.input); !errors.As(err, &validation) || validation.Fields["mentionIdentityIds"] != conversationaction.ValidationMentionIdentityIDsInvalid {
				t.Fatalf("%s: %v", test.name, err)
			}
		}
		for _, target := range []string{f.owner.OrganizationIdentity.ID, uuid.NewV7().String()} {
			_, err := send.Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{
				ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(),
				Body: "看下", Visibility: domain.MessageVisibilityInternalOnly, MentionIdentityIDs: []string{target},
			})
			var conflict *conversationaction.ConflictError
			if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonNoteMentionTargetInvalid {
				t.Fatalf("mention %s = %v", target, err)
			}
		}
	})

	t.Run("重试时提醒成员变化产生冲突", func(t *testing.T) {
		replay, err := send.Execute(ctx, f.owner, noteInput)
		if err != nil || replay.ID != note.ID || len(replay.Mentions) != 1 {
			t.Fatalf("replay=%+v err=%v", replay, err)
		}
		changed := noteInput
		changed.MentionIdentityIDs = nil
		var conflict *conversationaction.ConflictError
		if _, err := send.Execute(ctx, f.owner, changed); !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonIdempotencyMismatch {
			t.Fatalf("changed mentions retry = %v", err)
		}
	})

	t.Run("周期关闭后移出@我的视图", func(t *testing.T) {
		// 关闭前留下一条未读提醒，关闭后不再计入客户会话提醒总数。
		if _, err := send.Execute(ctx, f.owner, conversationaction.CustomerTextMessageInput{
			ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(),
			Body: "@成员 结单前再确认下", Visibility: domain.MessageVisibilityInternalOnly, MentionIdentityIDs: []string{memberID},
		}); err != nil {
			t.Fatal(err)
		}
		if _, counts := customerRow(f.member, domain.CustomerInboxViewMentioned); counts.CustomerMentioned != 1 {
			t.Fatalf("unread mention before close counts=%+v", counts)
		}
		tasks := servertask.New(f.db, serverconfig.NATSConfig{})
		closeSession := conversationaction.NewCloseServiceSessionAction(f.db, agentrunaction.NewExecuteAction(f.db, tasks, nil, testAttachmentReader(f.db), nil))
		if _, err := closeSession.Execute(ctx, f.owner, f.conversationID); err != nil {
			t.Fatal(err)
		}
		if row, counts := customerRow(f.member, domain.CustomerInboxViewMentioned); row != nil || counts.CustomerMentioned != 0 {
			t.Fatalf("closed session still in mentioned view: %+v counts=%+v", row, counts)
		}
		// 客户再次来信开启新周期，上一周期的提醒不再归入@我的。
		if _, err := f.receive.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
			ChannelID: f.channelID, ExternalID: "web-session:0123456789abcdef0123456789abcdef", ConversationID: &f.conversationID,
			ClientMessageID: uuid.NewV7().String(), Body: "又有新问题",
		}); err != nil {
			t.Fatal(err)
		}
		if row, _ := customerRow(f.member, domain.CustomerInboxViewMentioned); row != nil {
			t.Fatalf("new session inherits previous mentions: %+v", row)
		}
	})
}
