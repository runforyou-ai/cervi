//go:build server

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type testDirectReplyRuntime struct {
	t        *testing.T
	expected *conversationaction.ConversationMessageReference
	calls    int
}

// Run 检查引用消息的模型输入并返回可辨认的测试回复。
func (r *testDirectReplyRuntime) Run(ctx context.Context, request agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
	t := r.t
	if request.CustomerHistorySearch != nil {
		t.Fatal("direct chat received customer history tool")
	}
	r.calls++
	triggers, err := feed.Peek(ctx, 0)
	if err != nil {
		return agentruntime.RunResult{}, err
	}
	claimed, err := feed.Claim(ctx, triggers[len(triggers)-1].Seq)
	if err != nil {
		return agentruntime.RunResult{}, err
	}
	last := claimed.Messages[len(claimed.Messages)-1]
	if r.expected == nil {
		if last.Content != "第一问" {
			t.Fatalf("ordinary content changed: %+v", last)
		}
	} else {
		var content struct {
			Body    string `json:"body"`
			ReplyTo struct {
				MessageID  string `json:"messageId"`
				SenderID   string `json:"senderIdentityId"`
				SenderName string `json:"senderName"`
				Body       string `json:"body"`
				Deleted    bool   `json:"deleted"`
			} `json:"replyTo"`
		}
		if err := json.Unmarshal([]byte(last.Content), &content); err != nil {
			t.Fatal(err)
		}
		if last.Role != agentruntime.MessageRoleUser || content.Body != "针对引用提问" || content.ReplyTo.MessageID != r.expected.ID || content.ReplyTo.Deleted != r.expected.Deleted || content.ReplyTo.Body != r.expected.Body {
			t.Fatalf("claimed reference=%+v expected=%+v", content, r.expected)
		}
		if r.expected.Deleted {
			if content.ReplyTo.SenderID != "" || content.ReplyTo.SenderName != "" || strings.Contains(last.Content, "早期答案") {
				t.Fatalf("deleted reference leaked: %s", last.Content)
			}
		} else if content.ReplyTo.SenderID != r.expected.Sender.SourceID || content.ReplyTo.SenderName != *r.expected.Sender.DisplayName {
			t.Fatalf("reference sender=%+v", content.ReplyTo)
		}
		if len(claimed.Messages) != 100 {
			t.Fatalf("history window=%d", len(claimed.Messages))
		}
		for _, message := range claimed.Messages[:len(claimed.Messages)-1] {
			if message.Content == "早期答案" {
				t.Fatal("reference target unexpectedly within history window")
			}
		}
	}
	answer := "早期答案"
	if r.expected != nil {
		answer = "后续答案"
	}
	return agentruntime.RunResult{Content: answer, EndSeq: claimed.EndSeq}, nil
}

// testAgentDirectReplies 验证历史窗口外的引用、引用删除和连续引用进入真实 Agent 输入流。
func testAgentDirectReplies(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string) {
	t.Helper()
	ctx := context.Background()
	created, err := agentaction.NewCreateAgentAction(db).Execute(ctx, identity, agentaction.CreateInput{
		DisplayName: "引用助手", RoleID: roleID,
		Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{
			ProviderID: providerID, ModelIdentifier: modelID, SystemInstruction: "结合引用回答问题",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tasks := servertask.New(db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	scheduler := agentrunaction.NewScheduler(tasks)
	first, err := conversationaction.NewSendFirstAgentTextMessageAction(db, scheduler).Execute(ctx, identity, conversationaction.FirstAgentTextMessageInput{ConversationID: uuid.NewV7().String(),
		AgentIdentityID: created.IdentityID, ClientMessageID: uuid.NewV7().String(), Body: "第一问",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Conversation.Agent.PreviewSenderIdentityType == nil || *first.Conversation.Agent.PreviewSenderIdentityType != domain.OrganizationIdentityTypeUser {
		t.Fatalf("first message preview identity = %#v", first.Conversation)
	}
	// 运行状态与成功回复从同一 AI 身份读取头像。
	avatarID := uuid.NewV7().String()
	if _, err := db.NewUpdate().Model((*servermodels.OrganizationIdentity)(nil)).Set("avatar_file_id = ?", avatarID).Where("id = ?", created.IdentityID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	queuedHistory, err := conversationaction.NewListConversationMessagesQuery(db).Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: first.Conversation.ID})
	if err != nil || queuedHistory.LatestAgentRun == nil || queuedHistory.LatestAgentRun.AgentAvatarFileID == nil || *queuedHistory.LatestAgentRun.AgentAvatarFileID != avatarID {
		t.Fatalf("queued agent avatar = %#v, error = %v", queuedHistory.LatestAgentRun, err)
	}
	runtime := &testDirectReplyRuntime{t: t}
	execute := agentrunaction.NewExecuteAction(db, tasks, runtime, nil)
	// 同步执行真实排队记录，用可控 Runtime 检查模型接收的上下文。
	runNext := func() *servermodels.AgentRun {
		t.Helper()
		run := &servermodels.AgentRun{}
		if err := db.NewSelect().Model(run).Where("agr.conversation_id = ? AND agr.status = ?", first.Conversation.ID, domain.AgentRunStatusQueued).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if err := execute.Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
			t.Fatal(err)
		}
		if err := db.NewSelect().Model(run).WherePK().Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if run.ResponseMessageID == nil || run.Status != string(domain.AgentRunStatusSucceeded) {
			t.Fatalf("run=%+v", run)
		}
		return run
	}
	initialRun := runNext()
	history, err := conversationaction.NewListConversationMessagesQuery(db).Execute(ctx, identity, conversationaction.ConversationMessageHistoryInput{ConversationID: first.Conversation.ID})
	if err != nil || len(history.Messages) != 2 || history.Messages[0].Sender == nil || history.Messages[0].Sender.IdentityType == nil || *history.Messages[0].Sender.IdentityType != domain.OrganizationIdentityTypeUser || history.Messages[1].Sender == nil || history.Messages[1].Sender.IdentityType == nil || *history.Messages[1].Sender.IdentityType != domain.OrganizationIdentityTypeAgent {
		t.Fatalf("message sender identities = %#v, error = %v", history.Messages, err)
	}
	if history.LatestAgentRun == nil || history.LatestAgentRun.AgentAvatarFileID == nil || *history.LatestAgentRun.AgentAvatarFileID != avatarID || history.Messages[1].Sender == nil || history.Messages[1].Sender.AvatarFileID == nil || *history.Messages[1].Sender.AvatarFileID != avatarID {
		t.Fatalf("completed agent avatars = %#v, sender = %#v", history.LatestAgentRun, history.Messages[1].Sender)
	}
	send := conversationaction.NewSendAgentTextMessageAction(db, scheduler)
	for i := range 101 {
		if _, err := send.Execute(ctx, identity, conversationaction.InternalTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: fmt.Sprintf("普通消息 %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	input := conversationaction.InternalTextMessageInput{ConversationID: first.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "针对引用提问", ReplyToMessageID: *initialRun.ResponseMessageID}
	reply, err := send.Execute(ctx, identity, input)
	if err != nil {
		t.Fatal(err)
	}
	if reply.ReplyTo == nil || reply.ReplyTo.Sender == nil || reply.ReplyTo.Sender.IdentityType == nil || *reply.ReplyTo.Sender.IdentityType != domain.OrganizationIdentityTypeAgent || reply.Sender == nil || reply.Sender.IdentityType == nil || *reply.Sender.IdentityType != domain.OrganizationIdentityTypeUser {
		t.Fatalf("reply sender identities = %#v", reply)
	}
	runtime.expected = reply.ReplyTo
	if replay, err := send.Execute(ctx, identity, input); err != nil || replay.ID != reply.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	triggerCount, err := db.NewSelect().Model((*servermodels.ConversationAgentTrigger)(nil)).Where("cat.trigger_message_id = ?", reply.ID).Count(ctx)
	if err != nil || triggerCount != 1 {
		t.Fatalf("replayed trigger count=%d err=%v", triggerCount, err)
	}
	runNext()
	// 已保存的引用在执行前被删除时，不再向模型提供被删除正文。
	input.ClientMessageID = uuid.NewV7().String()
	if _, err := send.Execute(ctx, identity, input); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id = ?", input.ReplyToMessageID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	runtime.expected = &conversationaction.ConversationMessageReference{ID: input.ReplyToMessageID, Deleted: true}
	runNext()
	// 引用一条带引用的用户消息时，仅携带该条正文，不递归展开原引用。
	input.ClientMessageID, input.ReplyToMessageID = uuid.NewV7().String(), reply.ID
	chained, err := send.Execute(ctx, identity, input)
	if err != nil {
		t.Fatal(err)
	}
	runtime.expected = chained.ReplyTo
	runNext()
	// 用窗口外附件验证模型引用摘要；附件内容本身不进入文本上下文。
	if _, err := db.NewUpdate().Model((*servermodels.Message)(nil)).Set("type = ?", domain.MessageTypeAttachment).Set("body = ''").Where("id = ?", first.Message.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewRaw(`INSERT INTO message_attachments (message_id, organization_id, name, content_type, byte_size, upload_status)
 VALUES (?, ?, 'reference.txt', 'text/plain', 0, ?)`, first.Message.ID, identity.Organization.ID, domain.AttachmentReady).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"", "附件说明", "取消前说明"} {
		if _, err := db.NewUpdate().Model((*servermodels.Message)(nil)).Set("body = ?", body).Where("id = ?", first.Message.ID).Exec(ctx); err != nil {
			t.Fatal(err)
		}
		input.ClientMessageID, input.ReplyToMessageID = uuid.NewV7().String(), first.Message.ID
		attachmentReply, err := send.Execute(ctx, identity, input)
		if err != nil {
			t.Fatal(err)
		}
		expectedBody := body
		if body == "" {
			expectedBody = "reference.txt"
		}
		if attachmentReply.ReplyTo == nil || attachmentReply.ReplyTo.Body != expectedBody {
			t.Fatalf("attachment reference=%+v", attachmentReply.ReplyTo)
		}
		runtime.expected = attachmentReply.ReplyTo
		if body == "取消前说明" {
			if _, err := db.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id = ?", first.Message.ID).Exec(ctx); err != nil {
				t.Fatal(err)
			}
			runtime.expected = &conversationaction.ConversationMessageReference{ID: first.Message.ID, Type: domain.MessageTypeAttachment, Deleted: true}
		}
		runNext()
	}
	if runtime.calls != 7 {
		t.Fatalf("runtime calls=%d", runtime.calls)
	}
}
