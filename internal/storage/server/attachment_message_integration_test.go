//go:build server

package server

import (
	"context"
	"errors"
	"testing"
	"uuid"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestAttachmentMessages 验证附件激活、发送幂等、成员资格、首发单聊和历史读取。
func TestAttachmentMessages(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	upload := fileaction.NewCreateUploadAction(f.db)
	send := conversationaction.NewSendAttachmentMessageAction(f.db)
	query := conversationaction.NewListConversationMessagesQuery(f.db)
	for _, target := range []string{"group", "direct"} {
		file, err := upload.Execute(ctx, f.owner, domain.FileStorageBackendLocal, fileaction.UploadInput{
			Purpose: domain.FilePurposeMessageAttachment, FileName: "附件 " + target + ".arbitrary", ContentType: "", ByteSize: domain.FilePartSize + 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		if file.PartSize != domain.FilePartSize || file.ContentType != "application/octet-stream" {
			t.Fatalf("file=%+v", file)
		}
		input := conversationaction.AttachmentMessageInput{ConversationID: f.groupID, FileID: file.ID, ClientMessageID: uuid.NewV7().String()}
		if _, err := send.Execute(ctx, f.owner, input); !errors.Is(err, fileaction.ErrFileNotFound) {
			t.Fatalf("pending upload accepted: %v", err)
		}
		if _, err := fileaction.NewMarkUploadedAction(f.db).Execute(ctx, f.owner, file.ID, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := send.Execute(ctx, f.member, input); !errors.Is(err, fileaction.ErrFileNotFound) {
			t.Fatalf("another uploader accepted: %v", err)
		}
		if target == "direct" {
			input.ConversationID = ""
			input.TargetIdentityID = f.member.OrganizationIdentity.ID
		}
		result, err := send.Execute(ctx, f.owner, input)
		if err != nil {
			t.Fatal(err)
		}
		if result.Message.Type != domain.MessageTypeAttachment || result.Message.Body != "" || result.Message.Attachment == nil || result.Message.Attachment.ID != file.ID {
			t.Fatalf("result=%+v", result)
		}
		repeated, err := send.Execute(ctx, f.owner, input)
		if err != nil || repeated.Message.ID != result.Message.ID || repeated.Message.Attachment == nil {
			t.Fatalf("repeat=%+v err=%v", repeated, err)
		}
		changed := input
		changed.FileID = uuid.NewV7().String()
		var conflict *conversationaction.ConflictError
		if _, err := send.Execute(ctx, f.owner, changed); !errors.As(err, &conflict) {
			t.Fatalf("changed intent=%v", err)
		}
		file = &servermodels.File{ID: file.ID}
		if err := f.db.NewSelect().Model(file).WherePK().Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if file.Status != string(domain.FileStatusActive) || file.ExpiresAt != nil {
			t.Fatalf("file not activated: %+v", file)
		}
		history, err := query.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: result.ConversationID, AroundMessageID: result.Message.ID})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, message := range history.Messages {
			if message.ID == result.Message.ID && message.Attachment != nil && message.Attachment.Name == file.OriginalName {
				found = true
			}
		}
		if !found {
			t.Fatal("attachment missing from history")
		}
		downloaded, err := query.GetAttachmentFile(ctx, f.member, result.ConversationID, result.Message.ID)
		if err != nil || downloaded.ID != file.ID {
			t.Fatalf("download=%+v err=%v", downloaded, err)
		}
		items, _, err := inboxaction.NewLoadInboxQuery(f.db).Execute(ctx, f.member, inboxaction.LoadInput{Scope: domain.InboxScopeAll})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			if item.ID != result.ConversationID {
				continue
			}
			if target == "direct" && (item.Direct == nil || item.Direct.Preview == nil || *item.Direct.Preview != file.OriginalName) {
				t.Fatalf("direct preview=%+v", item.Direct)
			}
			if target == "group" && (item.Group == nil || item.Group.Preview == nil || *item.Group.Preview != file.OriginalName) {
				t.Fatalf("group preview=%+v", item.Group)
			}
		}
		if err := upload.Cancel(ctx, f.owner, file.ID); err != nil {
			t.Fatal(err)
		}
		active := &servermodels.File{ID: file.ID}
		if err := f.db.NewSelect().Model(active).WherePK().Scan(ctx); err != nil || active.Status != string(domain.FileStatusActive) {
			t.Fatalf("cancel removed sent file: %+v %v", active, err)
		}
	}
}

// TestAttachmentBatchLifecycle 验证先入库、顺序、接收方状态、幂等和取消。
func TestAttachmentBatchLifecycle(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	upload := fileaction.NewCreateUploadAction(f.db)
	send := conversationaction.NewSendAttachmentMessageAction(f.db)
	query := conversationaction.NewListConversationMessagesQuery(f.db)
	input := conversationaction.AttachmentBatchInput{BatchID: uuid.NewV7().String(), TargetIdentityID: f.member.OrganizationIdentity.ID, Body: "两份文件的说明"}
	for index := 0; index < 2; index++ {
		file, err := upload.Execute(ctx, f.owner, domain.FileStorageBackendLocal, fileaction.UploadInput{Purpose: domain.FilePurposeMessageAttachment, FileName: "photo.png", ContentType: "image/png", ByteSize: domain.FilePartSize + 1})
		if err != nil {
			t.Fatal(err)
		}
		input.Attachments = append(input.Attachments, conversationaction.AttachmentMessageInput{FileID: file.ID, ClientMessageID: uuid.NewV7().String(), ImageWidth: 320, ImageHeight: 200})
	}
	result, err := send.ExecuteBatch(ctx, f.owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 3 || result.Messages[2].Body != input.Body {
		t.Fatalf("batch=%+v", result)
	}
	for index := 0; index < 2; index++ {
		message := result.Messages[index]
		if message.Attachment.UploadStatus != domain.AttachmentUploading || message.Attachment.ID != input.Attachments[index].FileID {
			t.Fatalf("pending=%+v", message)
		}
		next := result.Messages[index+1]
		if message.OriginatedAt.After(next.OriginatedAt) || (message.OriginatedAt.Equal(next.OriginatedAt) && message.ID >= next.ID) {
			t.Fatal("message order was not preserved")
		}
		if _, err := query.GetAttachmentFile(ctx, f.member, result.ConversationID, message.ID); err == nil {
			t.Fatal("incomplete attachment downloadable")
		}
	}
	repeated, err := send.ExecuteBatch(ctx, f.owner, input)
	if err != nil || len(repeated.Messages) != 3 || repeated.Messages[0].ID != result.Messages[0].ID {
		t.Fatalf("repeat=%+v %v", repeated, err)
	}
	changed := input
	changed.Body = "不同说明"
	var conflict *conversationaction.ConflictError
	if _, err := send.ExecuteBatch(ctx, f.owner, changed); !errors.As(err, &conflict) {
		t.Fatalf("batch intent changed: %v", err)
	}
	first := input.Attachments[0].FileID
	second := input.Attachments[1].FileID
	// 通用临时文件取消接口不能绕过消息状态撤去已入库附件。
	if err := upload.Cancel(ctx, f.owner, first); err != nil {
		t.Fatal(err)
	}
	pending := &servermodels.File{ID: first}
	if err := f.db.NewSelect().Model(pending).WherePK().Scan(ctx); err != nil || pending.Status != string(domain.FileStatusPending) {
		t.Fatalf("attached file was cancelled directly: %+v %v", pending, err)
	}
	if err := send.UpdateUploads(ctx, f.member, []string{first}, domain.AttachmentReady); err == nil {
		t.Fatal("recipient changed upload")
	}
	if err := send.UpdateUploads(ctx, f.owner, []string{first}, domain.AttachmentReady); err == nil {
		t.Fatal("pending file marked ready")
	}
	if err := send.UpdateUploads(ctx, f.owner, []string{first}, domain.AttachmentFailed); err != nil {
		t.Fatal(err)
	}
	if err := send.UpdateUploads(ctx, f.owner, []string{first}, domain.AttachmentUploading); err != nil {
		t.Fatal(err)
	}
	if _, err := fileaction.NewMarkUploadedAction(f.db).Execute(ctx, f.owner, first, ""); err != nil {
		t.Fatal(err)
	}
	if err := send.UpdateUploads(ctx, f.owner, []string{first}, domain.AttachmentReady); err != nil {
		t.Fatal(err)
	}
	if _, err := query.GetAttachmentFile(ctx, f.member, result.ConversationID, result.Messages[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := send.UpdateUploads(ctx, f.owner, []string{second}, domain.AttachmentCancelled); err != nil {
		t.Fatal(err)
	}
	if err := send.UpdateUploads(ctx, f.owner, []string{second}, domain.AttachmentReady); err != nil {
		t.Fatal(err)
	}
	history, err := query.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: result.ConversationID})
	if err != nil {
		t.Fatal(err)
	}
	foundFirst, foundCaption := false, false
	for _, message := range history.Messages {
		if message.ID == result.Messages[1].ID {
			t.Fatal("cancelled attachment visible")
		}
		if message.ID == result.Messages[0].ID {
			foundFirst = message.Attachment.UploadStatus == domain.AttachmentReady
		}
		if message.ID == result.Messages[2].ID {
			foundCaption = true
		}
	}
	states, err := query.AttachmentStates(ctx, f.member, result.ConversationID, []string{result.Messages[0].ID, result.Messages[1].ID})
	if err != nil || len(states) != 2 {
		t.Fatalf("states=%+v %v", states, err)
	}
	for _, state := range states {
		if state.MessageID == result.Messages[0].ID && state.Attachment.UploadStatus != domain.AttachmentReady {
			t.Fatal("ready state missing")
		}
		if state.MessageID == result.Messages[1].ID && !state.Deleted {
			t.Fatal("cancelled state missing")
		}
	}
	if !foundFirst || !foundCaption {
		t.Fatalf("history=%+v", history.Messages)
	}
	// 完成先提交时，随后到达的取消仍须撤去消息，迟到完成不能恢复它。
	if err := send.UpdateUploads(ctx, f.owner, []string{first}, domain.AttachmentCancelled); err != nil {
		t.Fatal(err)
	}
	if err := send.UpdateUploads(ctx, f.owner, []string{first}, domain.AttachmentReady); err != nil {
		t.Fatal(err)
	}
	states, err = query.AttachmentStates(ctx, f.member, result.ConversationID, []string{result.Messages[0].ID})
	if err != nil || len(states) != 1 || !states[0].Deleted || states[0].Attachment.UploadStatus != domain.AttachmentCancelled {
		t.Fatalf("completed attachment cancellation=%+v %v", states, err)
	}
	if _, err := query.GetAttachmentFile(ctx, f.member, result.ConversationID, result.Messages[0].ID); !errors.Is(err, fileaction.ErrFileNotFound) {
		t.Fatalf("cancelled attachment download=%v", err)
	}
	groupInput := input
	groupInput.BatchID = uuid.NewV7().String()
	groupInput.ConversationID = f.groupID
	groupInput.TargetIdentityID = ""
	if _, err := send.ExecuteBatch(ctx, f.owner, groupInput); err == nil {
		t.Fatal("group batch accepted")
	}
}

// TestCancelOnlyAttachment 验证取消唯一一条附件时清空会话摘要且保留游标记录。
func TestCancelOnlyAttachment(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	upload := fileaction.NewCreateUploadAction(f.db)
	send := conversationaction.NewSendAttachmentMessageAction(f.db)
	file, err := upload.Execute(ctx, f.owner, domain.FileStorageBackendLocal, fileaction.UploadInput{Purpose: domain.FilePurposeMessageAttachment, FileName: "empty", ByteSize: 0})
	if err != nil {
		t.Fatal(err)
	}
	result, err := send.ExecuteBatch(ctx, f.owner, conversationaction.AttachmentBatchInput{BatchID: uuid.NewV7().String(), TargetIdentityID: f.member.OrganizationIdentity.ID, Attachments: []conversationaction.AttachmentMessageInput{{FileID: file.ID, ClientMessageID: uuid.NewV7().String()}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := send.UpdateUploads(ctx, f.owner, []string{file.ID}, domain.AttachmentCancelled); err != nil {
		t.Fatal(err)
	}
	conversation := &servermodels.Conversation{ID: result.ConversationID}
	if err := f.db.NewSelect().Model(conversation).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if conversation.LastMessageID != nil || conversation.LastMessageAt != nil {
		t.Fatalf("summary=%+v", conversation)
	}
}
