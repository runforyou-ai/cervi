//go:build server

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/api"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

// TestMessageSequenceCommitOrder 验证所有会话类型在提交前阻塞后续分配，回滚允许编号重用。
func TestMessageSequenceCommitOrder(t *testing.T) {
	f := newNavigationFixture(t)
	for _, kind := range []domain.ConversationType{domain.ConversationTypeDirect, domain.ConversationTypeAgent, domain.ConversationTypeGroup, domain.ConversationTypeCustomer} {
		for _, rollback := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/rollback=%t", kind, rollback), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cv := &servermodels.Conversation{ID: uuid.NewV7().String(), OrganizationID: f.owner.Organization.ID, Type: string(kind), Status: string(domain.ConversationStatusActive)}
				if _, err := f.db.NewInsert().Model(cv).Column("id", "organization_id", "type", "status").Exec(ctx); err != nil {
					t.Fatal(err)
				}
				tx, err := f.db.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				if err := tx.NewSelect().Model(cv).WherePK().For("UPDATE").Scan(ctx); err != nil {
					t.Fatal(err)
				}
				first, _, err := chatstate.AppendMessage(ctx, tx, cv, &servermodels.Message{ID: uuid.NewV7().String(), OrganizationID: cv.OrganizationID, ConversationID: cv.ID, Type: "system", Body: "先取锁", OriginatedAt: time.Now().UTC()})
				if err != nil || first.MessageSeq != 1 {
					t.Fatalf("first=%+v err=%v", first, err)
				}
				done := make(chan error, 1)
				var second *servermodels.Message
				go func() {
					done <- f.db.RunInTx(ctx, nil, func(ctx context.Context, next bun.Tx) error {
						locked := &servermodels.Conversation{ID: cv.ID}
						if err := next.NewSelect().Model(locked).WherePK().For("UPDATE").Scan(ctx); err != nil {
							return err
						}
						var err error
						second, _, err = chatstate.AppendMessage(ctx, next, locked, &servermodels.Message{ID: uuid.NewV7().String(), OrganizationID: cv.OrganizationID, ConversationID: cv.ID, Type: "system", Body: "后取锁但来源更早", OriginatedAt: first.OriginatedAt.Add(-time.Hour)})
						return err
					})
				}()
				waitConversationLock(t, ctx, f.db, cv.ID)
				if count, err := f.db.NewSelect().Model((*servermodels.Message)(nil)).Where("conversation_id = ?", cv.ID).Count(ctx); err != nil || count != 0 {
					t.Fatalf("uncommitted count=%d err=%v", count, err)
				}
				want := int64(2)
				if rollback {
					err = tx.Rollback()
					want = 1
				} else {
					err = tx.Commit()
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := waitChatResult(t, ctx, done); err != nil {
					t.Fatal(err)
				}
				if second.MessageSeq != want {
					t.Fatalf("sequence=%d want=%d", second.MessageSeq, want)
				}
				if err := f.db.NewSelect().Model(cv).WherePK().Scan(ctx); err != nil {
					t.Fatal(err)
				}
				if cv.LastMessageSeq != want || cv.LastMessageID == nil || *cv.LastMessageID != second.ID {
					t.Fatalf("summary=%+v", cv)
				}
				if count, err := f.db.NewSelect().Model((*servermodels.Message)(nil)).Where("conversation_id = ?", cv.ID).Count(ctx); err != nil || int64(count) != want {
					t.Fatalf("visible count=%d err=%v", count, err)
				}
			})
		}
	}
}

// TestMessageSequenceLargeReadAndWindows 验证大整数序号的双向分页、引用定位、访客读取与删除后的阅读基线。
func TestMessageSequenceLargeReadAndWindows(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	const base int64 = 9007199254740991
	if _, err := f.db.NewUpdate().Model((*servermodels.Conversation)(nil)).Set("last_message_seq = ?", base).Where("id = ?", f.conversationID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	var sent []conversationaction.Message
	for range 3 {
		result, err := f.visitorMessage(ctx, "大整数消息")
		if err != nil {
			t.Fatal(err)
		}
		sent = append(sent, result.Message)
	}
	for i, message := range sent {
		if message.MessageSeq != base+int64(i)+1 {
			t.Fatalf("message=%+v", message)
		}
	}
	query := conversationaction.NewListConversationMessagesQuery(f.db)
	point := &conversationaction.MessageCursorPoint{ID: sent[1].ID, MessageSeq: sent[1].MessageSeq}
	after, err := query.Execute(ctx, f.owner, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID, After: point})
	if err != nil || len(after.Messages) != 1 || after.Messages[0].ID != sent[2].ID {
		t.Fatalf("after=%+v err=%v", after, err)
	}
	before, err := query.Execute(ctx, f.owner, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID, Before: point})
	if err != nil || len(before.Messages) != 2 || before.Messages[1].ID != sent[0].ID {
		t.Fatalf("before=%+v err=%v", before, err)
	}
	around, err := query.Execute(ctx, f.owner, conversationaction.ConversationMessageHistoryInput{ConversationID: f.conversationID, AroundMessageID: sent[1].ID})
	if err != nil || len(around.Messages) != 4 || around.Messages[2].MessageSeq != point.MessageSeq {
		t.Fatalf("around=%+v err=%v", around, err)
	}
	visitor, err := conversationaction.NewListWebsiteMessagesQuery(f.db).Execute(ctx, conversationaction.MessageHistoryInput{ChannelID: f.channelID, ExternalID: "web-session:0123456789abcdef0123456789abcdef", ConversationID: f.conversationID, After: point})
	if err != nil || len(visitor.Messages) != 1 || visitor.Messages[0].MessageSeq != sent[2].MessageSeq {
		t.Fatalf("visitor=%+v err=%v", visitor, err)
	}
	read := conversationaction.NewMarkConversationReadAction(f.db)
	for _, message := range []conversationaction.Message{sent[1], sent[0], sent[1]} {
		result, err := read.Execute(ctx, f.owner, f.conversationID, message.ID, false)
		if err != nil || result.ReadSeq != sent[1].MessageSeq {
			t.Fatalf("read=%+v err=%v", result, err)
		}
	}
	if _, err := f.db.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id = ?", sent[1].ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if row := f.inboxRow(t, f.owner, domain.CustomerInboxViewQueue); row.UnreadCount != 1 {
		t.Fatalf("deleted read baseline=%+v", row)
	}
	result, err := read.Execute(ctx, f.owner, f.conversationID, sent[2].ID, false)
	if err != nil || result.ReadSeq != sent[2].MessageSeq {
		t.Fatalf("next read=%+v err=%v", result, err)
	}
}

// TestMessageSequenceSummaryRecompute 验证重算按本地顺序选择摘要且已提交序号不回退。
func TestMessageSequenceSummaryRecompute(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	first := f.send(t, f.owner, "来源较新", false)
	second := f.send(t, f.owner, "本地较新", false)
	third := f.send(t, f.owner, "准备撤去", false)
	if _, err := f.db.NewUpdate().Model((*servermodels.Message)(nil)).Set("originated_at = ?", first.OriginatedAt.Add(-time.Hour)).Where("id = ?", second.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		cv := &servermodels.Conversation{ID: f.groupID}
		if err := tx.NewSelect().Model(cv).WherePK().For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id = ?", third.ID).Exec(ctx); err != nil {
			return err
		}
		return chatstate.RecomputeConversationSummary(ctx, tx, cv, third.ID)
	}); err != nil {
		t.Fatal(err)
	}
	cv := &servermodels.Conversation{ID: f.groupID}
	if err := f.db.NewSelect().Model(cv).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if cv.LastMessageID == nil || *cv.LastMessageID != second.ID || cv.LastMessageSeq != third.MessageSeq {
		t.Fatalf("recomputed=%+v", cv)
	}
	next := f.send(t, f.owner, "撤去后的消息", false)
	if next.MessageSeq != third.MessageSeq+1 {
		t.Fatalf("reused committed sequence=%d", next.MessageSeq)
	}
	// 唯一约束拒绝同一会话复用已提交序号。
	duplicate := &servermodels.Message{ID: uuid.NewV7().String(), OrganizationID: f.owner.Organization.ID, ConversationID: f.groupID, MessageSeq: next.MessageSeq, Type: "text", Body: "重复", OriginatedAt: time.Now()}
	_, err := f.db.NewInsert().Model(duplicate).Column("id", "organization_id", "conversation_id", "message_seq", "type", "body", "originated_at").Exec(ctx)
	if !pgerr.UniqueViolationOn(err, "messages_message_seq_unique") {
		t.Fatalf("duplicate accepted: %v", err)
	}
}

// TestMessageSequenceHTTPContract 验证真实数据库经过应用服务与 HTTP 后仍以字符串传输大整数序号。
func TestMessageSequenceHTTPContract(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	if _, err := f.db.NewUpdate().Model((*servermodels.Conversation)(nil)).Set("last_message_seq = 9007199254740992").Where("id = ?", f.conversationID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	sent, err := f.visitorMessage(ctx, "HTTP 大整数")
	if err != nil {
		t.Fatal(err)
	}
	login, err := authaction.NewLoginAction(f.db).Execute(ctx, authaction.LoginInput{OrganizationID: f.owner.Organization.ID, Email: "owner@navigation.test", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	backend := appservice.NewDirectBackend(f.db, nil, NewTenantResolver(f.db), nil, nil, nil)
	service := api.NewService(appservice.New(backend))
	for _, route := range []string{"messages", "read"} {
		t.Run(route, func(t *testing.T) {
			method := http.MethodGet
			var body bytes.Buffer
			if route == "read" {
				method = http.MethodPost
				if err := json.NewEncoder(&body).Encode(appservice.MarkConversationReadInput{LastReadMessageID: sent.Message.ID}); err != nil {
					t.Fatal(err)
				}
			}
			request := httptest.NewRequest(method, "/conversations/"+f.conversationID+"/"+route, &body)
			request = request.WithContext(tenant.WithAccessHost(request.Context(), f.owner.Organization.AccessHost))
			request.Header.Set("Authorization", "Bearer "+login.Token)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			service.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			key := "messageSeq"
			if route == "read" {
				key = "readSeq"
			}
			if !strings.Contains(response.Body.String(), `"`+key+`":"9007199254740993"`) {
				t.Fatalf("lossy response: %s", response.Body.String())
			}
		})
	}
	visitor := appservice.NewWebsiteVisitorDirectBackend(f.db, nil)
	page, err := visitor.ListMessages(ctx, appservice.WebsiteVisitorMeta{}, f.channelID, "web-session:0123456789abcdef0123456789abcdef", f.conversationID, appservice.WebsiteVisitorMessageHistoryInput{})
	if err != nil || page.Messages[len(page.Messages)-1].MessageSeq != "9007199254740993" {
		t.Fatalf("visitor=%+v err=%v", page, err)
	}
	next, err := visitor.ListMessages(ctx, appservice.WebsiteVisitorMeta{}, f.channelID, "web-session:0123456789abcdef0123456789abcdef", f.conversationID, appservice.WebsiteVisitorMessageHistoryInput{After: *page.After})
	if err != nil || len(next.Messages) != 0 {
		t.Fatalf("cursor=%+v err=%v", next, err)
	}
}
