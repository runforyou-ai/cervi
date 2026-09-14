//go:build server

package integrationtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	aiprovideraction "github.com/runforyou-ai/cervi/internal/actions/aiprovider"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type testAttachmentFiles struct{}

// Open 以原件名称生成可辨认的文件内容。
func (testAttachmentFiles) Open(_ context.Context, file *servermodels.File) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("content:" + file.OriginalName)), nil
}

// testAttachmentReader 创建以 http 生成附件链接、按原件名称返回内容的附件读取器。
func testAttachmentReader(db *bun.DB) *agentrunaction.AttachmentReader {
	return agentrunaction.NewAttachmentReader(db, testAttachmentFiles{}, "http")
}

type contextAttachmentContent struct {
	Body       string `json:"body"`
	Attachment struct {
		MessageID   string `json:"messageId"`
		Name        string `json:"name"`
		ContentType string `json:"contentType"`
		URL         string `json:"url"`
	} `json:"attachment"`
}

// TestAgentAttachmentInputs 验证 AI 聊天附件整批上传完成后触发一次运行，且上下文携带附件链接和可直传内容。
func TestAgentAttachmentInputs(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	provider, err := aiprovideraction.NewCreateAIProviderAction(f.db).Execute(ctx, f.owner, aiprovideraction.Input{
		Brand: domain.AIProviderBrandOpenAI, Name: uuid.NewV7().String(), APIKey: "test-key", APIURL: "https://models.test/v1",
		Models: []aiprovideraction.Model{{
			Identifier: "vision", Name: "视觉模型", Type: domain.AIModelTypeChat,
			InputModalities: []domain.AIModelInputModality{domain.AIModelInputModalityText, domain.AIModelInputModalityImage}, ContextWindow: 32000, MaxOutputTokens: 4096,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var roleID string
	if err := f.db.NewSelect().Table("roles").Column("id").Where("organization_id = ? AND kind = ?", f.owner.Organization.ID, domain.RoleKindCustomerService).Scan(ctx, &roleID); err != nil {
		t.Fatal(err)
	}
	agent, err := agentaction.NewCreateAgentAction(f.db).Execute(ctx, f.owner, agentaction.CreateInput{
		DisplayName: "附件助手", RoleID: roleID,
		Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{
			ProviderID: provider.ID, ModelIdentifier: "vision", SystemInstruction: "阅读附件并回答",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var accessHost string
	if err := f.db.NewSelect().Table("organizations").Column("access_host").Where("id = ?", f.owner.Organization.ID).Scan(ctx, &accessHost); err != nil {
		t.Fatal(err)
	}
	tasks := servertask.New(f.db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	scheduler := agentrunaction.NewScheduler(tasks)
	send := conversationaction.NewSendAttachmentMessageAction(f.db, scheduler)
	conversationID := uuid.NewV7().String()
	// sendBatch 在 AI 聊天中发送一批待上传附件，最后一个附件携带说明。
	sendBatch := func(input conversationaction.AttachmentBatchInput, names []string, types []string, body string) conversationaction.AttachmentBatchResult {
		t.Helper()
		for index, name := range names {
			input.Attachments = append(input.Attachments, conversationaction.AttachmentBatchItem{
				File:            fileaction.UploadInput{FileName: name, ContentType: types[index], ByteSize: domain.FilePartSize + 1},
				ClientMessageID: uuid.NewV7().String(),
			})
		}
		input.Attachments[len(input.Attachments)-1].Body = body
		result, err := send.ExecuteBatch(ctx, f.owner, input, domain.FileStorageBackendLocal)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	// complete 标记附件内容已上传并确认完成状态。
	complete := func(message conversationaction.ConversationMessage, status domain.AttachmentUploadStatus) {
		t.Helper()
		if status == domain.AttachmentReady {
			if _, err := fileaction.NewMarkUploadedAction(f.db).Execute(ctx, f.owner, message.Attachment.ID, ""); err != nil {
				t.Fatal(err)
			}
		}
		if err := send.UpdateUploads(ctx, f.owner, []string{message.Attachment.ID}, status); err != nil {
			t.Fatal(err)
		}
	}
	// inputSources 按输入序号返回本会话 Agent 输入的来源消息。
	inputSources := func() []string {
		t.Helper()
		sources := make([]string, 0)
		if err := f.db.NewSelect().Model((*servermodels.AgentInput)(nil)).ColumnExpr("ai.source_message_id").
			Join("JOIN agent_lanes AS al ON al.id = ai.lane_id").
			Where("al.conversation_id = ?", conversationID).OrderExpr("ai.input_seq ASC").Scan(ctx, &sources); err != nil {
			t.Fatal(err)
		}
		return sources
	}
	// runQueued 用可控 Runtime 执行会话中排队的运行。
	runQueued := func(runtime testAgentRuntime) {
		t.Helper()
		run := &servermodels.AgentRun{}
		if err := f.db.NewSelect().Model(run).Where("agr.conversation_id = ? AND agr.status = ?", conversationID, domain.AgentRunStatusQueued).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if err := agentrunaction.NewExecuteAction(f.db, tasks, runtime, testAttachmentReader(f.db)).Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
			t.Fatal(err)
		}
	}

	// AI 聊天草稿以附件首发创建会话，上传完成前不触发运行。
	first := sendBatch(conversationaction.AttachmentBatchInput{ConversationID: conversationID, AgentIdentityID: agent.IdentityID},
		[]string{"photo.png", "spec.pdf"}, []string{"image/png", "application/pdf"}, "请看附件")
	if first.ConversationID != conversationID || first.AgentConversation == nil || first.AgentConversation.ID != conversationID || len(first.Messages) != 2 {
		t.Fatalf("first batch = %+v", first)
	}
	complete(first.Messages[0], domain.AttachmentReady)
	if sources := inputSources(); len(sources) != 0 {
		t.Fatalf("inputs before batch completed = %v", sources)
	}
	complete(first.Messages[1], domain.AttachmentReady)
	// 重复的完成请求不追加输入。
	if err := send.UpdateUploads(ctx, f.owner, []string{first.Messages[1].Attachment.ID}, domain.AttachmentReady); err != nil {
		t.Fatal(err)
	}
	if sources := inputSources(); len(sources) != 1 || sources[0] != first.Messages[1].ID {
		t.Fatalf("inputs after batch completed = %v", sources)
	}
	image, document := first.Messages[0], first.Messages[1]
	var claimed []agentruntime.Message
	var modalities []domain.AIModelInputModality
	var imageContent string
	var foreignErr error
	runQueued(testAgentRuntime{run: func(ctx context.Context, request agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		triggers, err := feed.Peek(ctx, 0)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		input, err := feed.Claim(ctx, triggers[len(triggers)-1].Seq)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		claimed, modalities = input.Messages, request.Model.InputModalities
		content, err := request.ReadAttachment(ctx, image.ID)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		imageContent = string(content)
		_, foreignErr = request.ReadAttachment(ctx, uuid.NewV7().String())
		return agentruntime.RunResult{Content: "已阅读附件", EndSeq: input.EndSeq}, nil
	}})
	if !slices.Contains(modalities, domain.AIModelInputModalityImage) || len(claimed) != 2 || imageContent != "content:photo.png" || foreignErr == nil {
		t.Fatalf("claimed=%+v modalities=%v image=%q foreignErr=%v", claimed, modalities, imageContent, foreignErr)
	}
	for index, expected := range []struct {
		message     conversationaction.ConversationMessage
		name        string
		contentType string
		body        string
	}{{image, "photo.png", "image/png", ""}, {document, "spec.pdf", "application/pdf", "请看附件"}} {
		var content contextAttachmentContent
		if err := json.Unmarshal([]byte(claimed[index].Content), &content); err != nil {
			t.Fatal(err)
		}
		if claimed[index].ID != expected.message.ID || content.Body != expected.body || content.Attachment.MessageID != expected.message.ID ||
			content.Attachment.Name != expected.name || content.Attachment.ContentType != expected.contentType ||
			!strings.HasPrefix(content.Attachment.URL, "http://"+accessHost+"/storage/") ||
			claimed[index].Media == nil || claimed[index].Media.MIMEType != expected.contentType {
			t.Fatalf("claimed attachment %d = %+v content=%+v", index, claimed[index], content)
		}
	}

	// 上传中的附件及引用中的该附件不进入上下文；其后的文字已触发运行，附件完成时不再追加更早来源的输入。
	pending := sendBatch(conversationaction.AttachmentBatchInput{ConversationID: conversationID}, []string{"notes.txt"}, []string{"text/plain"}, "")
	if _, err := conversationaction.NewSendAgentTextMessageAction(f.db, scheduler).Execute(ctx, f.owner, conversationaction.InternalTextMessageInput{
		ConversationID: conversationID, ClientMessageID: uuid.NewV7().String(), Body: "先回答文字", ReplyToMessageID: pending.Messages[0].ID,
	}); err != nil {
		t.Fatal(err)
	}
	runQueued(testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
		triggers, err := feed.Peek(ctx, 0)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		input, err := feed.Claim(ctx, triggers[len(triggers)-1].Seq)
		if err != nil {
			return agentruntime.RunResult{}, err
		}
		referenced := false
		for _, message := range input.Messages {
			if message.ID == pending.Messages[0].ID {
				return agentruntime.RunResult{}, errors.New("uploading attachment entered context")
			}
			var content struct {
				ReplyTo *struct {
					Attachment *contextAttachmentContent `json:"attachment"`
				} `json:"replyTo"`
			}
			if json.Unmarshal([]byte(message.Content), &content) != nil || content.ReplyTo == nil {
				continue
			}
			if content.ReplyTo.Attachment != nil {
				return agentruntime.RunResult{}, errors.New("uploading reply attachment entered context")
			}
			referenced = true
		}
		if !referenced {
			return agentruntime.RunResult{}, errors.New("reply reference missing from context")
		}
		return agentruntime.RunResult{Content: "文字已回答", EndSeq: input.EndSeq}, nil
	}})
	complete(pending.Messages[0], domain.AttachmentReady)
	if sources := inputSources(); len(sources) != 2 {
		t.Fatalf("inputs after late attachment = %v", sources)
	}

	// 批次中失败的附件同样视为结束，输入来源取最新一条已完成附件。
	last := sendBatch(conversationaction.AttachmentBatchInput{ConversationID: conversationID}, []string{"a.txt", "b.txt"}, []string{"text/plain", "text/plain"}, "")
	complete(last.Messages[0], domain.AttachmentReady)
	complete(last.Messages[1], domain.AttachmentFailed)
	if sources := inputSources(); len(sources) != 3 || sources[2] != last.Messages[0].ID {
		t.Fatalf("inputs after failed attachment = %v", sources)
	}

	// Agent 停用后完成上传只更新附件状态，不追加输入。
	inactive := sendBatch(conversationaction.AttachmentBatchInput{ConversationID: conversationID}, []string{"c.txt"}, []string{"text/plain"}, "")
	if _, err := f.db.NewUpdate().Table("agents").Set("status = ?", domain.UserStatusInactive).Where("identity_id = ?", agent.IdentityID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	complete(inactive.Messages[0], domain.AttachmentReady)
	if sources := inputSources(); len(sources) != 3 {
		t.Fatalf("inputs after inactive agent attachment = %v", sources)
	}
}
