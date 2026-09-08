//go:build server

package server

import (
	"context"
	"errors"
	"testing"
	"uuid"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/actions/filemaintenance"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/domain"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
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
		// 已完成附件在单聊和群聊中均可按文件名引用。
		var reply conversationaction.ConversationMessage
		if target == "group" {
			reply, err = conversationaction.NewSendGroupTextMessageAction(f.db).Execute(ctx, f.member, conversationaction.GroupTextMessageInput{ConversationID: result.ConversationID, ClientMessageID: uuid.NewV7().String(), Body: "收到附件", ReplyToMessageID: result.Message.ID})
		} else {
			reply, err = conversationaction.NewSendDirectTextMessageAction(f.db).Execute(ctx, f.member, conversationaction.InternalTextMessageInput{ConversationID: result.ConversationID, ClientMessageID: uuid.NewV7().String(), Body: "收到附件", ReplyToMessageID: result.Message.ID})
		}
		if err != nil || reply.ReplyTo == nil || reply.ReplyTo.Body != file.OriginalName {
			t.Fatalf("attachment reply=%+v err=%v", reply, err)
		}
		replyHistory, err := query.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: result.ConversationID, AroundMessageID: reply.ID})
		if err != nil {
			t.Fatal(err)
		}
		foundReply := false
		for _, message := range replyHistory.Messages {
			if message.ID == reply.ID {
				foundReply = message.ReplyTo != nil && message.ReplyTo.Body == file.OriginalName
			}
		}
		if !foundReply {
			t.Fatal("attachment reply missing from history")
		}
		if err := filemaintenance.NewCancelUploadAction(f.db).Execute(ctx, f.owner, file.ID); err != nil {
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
	send := conversationaction.NewSendAttachmentMessageAction(f.db)
	query := conversationaction.NewListConversationMessagesQuery(f.db)
	input := conversationaction.AttachmentBatchInput{TargetIdentityID: f.member.OrganizationIdentity.ID}
	for index := 0; index < 2; index++ {
		input.Attachments = append(input.Attachments, conversationaction.AttachmentBatchItem{File: fileaction.UploadInput{FileName: "photo.png", ContentType: "image/png", ByteSize: domain.FilePartSize + 1}, ClientMessageID: uuid.NewV7().String(), ImageWidth: 320, ImageHeight: 200})
	}
	input.Attachments[len(input.Attachments)-1].Body = "  文件说明  "
	result, err := send.ExecuteBatch(ctx, f.owner, input, domain.FileStorageBackendLocal)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 2 || result.Messages[0].Body != "" || result.Messages[1].Body != "文件说明" {
		t.Fatalf("batch=%+v", result)
	}
	for index := 0; index < 2; index++ {
		message := result.Messages[index]
		if message.Attachment.UploadStatus != domain.AttachmentUploading || message.Attachment.ID == "" {
			t.Fatalf("pending=%+v", message)
		}
		if index > 0 {
			previous := result.Messages[index-1]
			if message.OriginatedAt.Before(previous.OriginatedAt) || (message.OriginatedAt.Equal(previous.OriginatedAt) && message.ID <= previous.ID) {
				t.Fatal("message order was not preserved")
			}
		}
		if _, err := query.GetAttachmentFile(ctx, f.member, result.ConversationID, message.ID); err == nil {
			t.Fatal("incomplete attachment downloadable")
		}
	}
	repeated, err := send.ExecuteBatch(ctx, f.owner, input, domain.FileStorageBackendLocal)
	if err != nil || len(repeated.Messages) != 2 || repeated.Messages[0].ID != result.Messages[0].ID || repeated.Messages[1].Body != "文件说明" {
		t.Fatalf("repeat=%+v %v", repeated, err)
	}
	changed := input
	changed.Attachments = append([]conversationaction.AttachmentBatchItem(nil), input.Attachments...)
	changed.Attachments[1].Body = "不同说明"
	var conflict *conversationaction.ConflictError
	if _, err := send.ExecuteBatch(ctx, f.owner, changed, domain.FileStorageBackendLocal); !errors.As(err, &conflict) {
		t.Fatalf("batch intent changed: %v", err)
	}
	// 说明随附件读取，并成为首发摘要和收件箱摘要。
	if result.Conversation == nil || result.Conversation.Preview == nil || *result.Conversation.Preview != "文件说明" {
		t.Fatalf("caption summary=%+v", result.Conversation)
	}
	captionHistory, err := query.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: result.ConversationID})
	if err != nil || len(captionHistory.Messages) != 2 || captionHistory.Messages[1].Body != "文件说明" || captionHistory.Messages[1].Attachment == nil {
		t.Fatalf("caption history=%+v %v", captionHistory, err)
	}
	items, _, err := inboxaction.NewLoadInboxQuery(f.db).Execute(ctx, f.member, inboxaction.LoadInput{Scope: domain.InboxScopeAll})
	if err != nil {
		t.Fatal(err)
	}
	foundConversation := false
	for _, item := range items {
		if item.ID == result.ConversationID {
			foundConversation = true
			if item.Direct == nil || item.Direct.Preview == nil || *item.Direct.Preview != "文件说明" {
				t.Fatalf("caption inbox=%+v", item.Direct)
			}
		}
	}
	if !foundConversation {
		t.Fatal("caption conversation missing from inbox")
	}

	first := result.Messages[0].Attachment.ID
	second := result.Messages[1].Attachment.ID
	// 通用临时文件取消接口不能绕过消息状态撤去已入库附件。
	if err := filemaintenance.NewCancelUploadAction(f.db).Execute(ctx, f.owner, first); err != nil {
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
	foundFirst := false
	for _, message := range history.Messages {
		if message.ID == result.Messages[1].ID {
			t.Fatal("cancelled attachment visible")
		}
		if message.ID == result.Messages[0].ID {
			foundFirst = message.Attachment.UploadStatus == domain.AttachmentReady
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
	if !foundFirst || len(history.Messages) != 1 {
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
	groupInput.ConversationID = f.groupID
	groupInput.TargetIdentityID = ""
	if _, err := send.ExecuteBatch(ctx, f.owner, groupInput, domain.FileStorageBackendLocal); err == nil {
		t.Fatal("group batch accepted")
	}
}

// TestCancelOnlyAttachment 验证取消唯一一条附件时清空会话摘要且保留游标记录。
func TestCancelOnlyAttachment(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	send := conversationaction.NewSendAttachmentMessageAction(f.db)
	input := conversationaction.AttachmentBatchInput{TargetIdentityID: f.member.OrganizationIdentity.ID, Attachments: []conversationaction.AttachmentBatchItem{{File: fileaction.UploadInput{FileName: "empty", ByteSize: 0}, ClientMessageID: uuid.NewV7().String()}}}
	result, err := send.ExecuteBatch(ctx, f.owner, input, domain.FileStorageBackendLocal)
	if err != nil {
		t.Fatal(err)
	}
	if err := send.UpdateUploads(ctx, f.owner, []string{result.Messages[0].Attachment.ID}, domain.AttachmentCancelled); err != nil {
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

// TestAttachmentReplayAfterCleanup 验证取消和文件回收后的重放不创建文件或恢复消息。
func TestAttachmentReplayAfterCleanup(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	send := conversationaction.NewSendAttachmentMessageAction(f.db)
	input := conversationaction.AttachmentBatchInput{TargetIdentityID: f.member.OrganizationIdentity.ID, Attachments: []conversationaction.AttachmentBatchItem{{File: fileaction.UploadInput{FileName: "cancelled.txt", ByteSize: 7}, ClientMessageID: uuid.NewV7().String()}}}
	result, err := send.ExecuteBatch(ctx, f.owner, input, domain.FileStorageBackendLocal)
	if err != nil {
		t.Fatal(err)
	}
	fileID := result.Messages[0].Attachment.ID
	if err := send.UpdateUploads(ctx, f.owner, []string{fileID}, domain.AttachmentCancelled); err != nil {
		t.Fatal(err)
	}
	local, err := serverfilecontent.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := filemaintenance.NewDeleteExpiredAction(f.db, serverfilecontent.NewDeleter(local, nil)).Execute(ctx, filemaintenance.DeleteExpiredInput{FileID: fileID}); err != nil {
		t.Fatal(err)
	}
	repeated, err := send.ExecuteBatch(ctx, f.owner, input, domain.FileStorageBackendLocal)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.Messages[0].ID != result.Messages[0].ID || repeated.Messages[0].Attachment.ID != "" || repeated.Messages[0].Attachment.UploadStatus != domain.AttachmentCancelled {
		t.Fatalf("replayed=%+v", repeated)
	}
	count, err := f.db.NewSelect().Model((*servermodels.File)(nil)).Where("organization_id = ?", f.owner.Organization.ID).Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("new file created: %d %v", count, err)
	}
}

// TestAttachmentBatchRollback 验证后续消息冲突时撤销本次新建的文件和消息。
func TestAttachmentBatchRollback(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	send := conversationaction.NewSendAttachmentMessageAction(f.db)
	original := conversationaction.AttachmentBatchItem{File: fileaction.UploadInput{FileName: "first.txt", ByteSize: 7}, ClientMessageID: uuid.NewV7().String()}
	input := conversationaction.AttachmentBatchInput{TargetIdentityID: f.member.OrganizationIdentity.ID, Attachments: []conversationaction.AttachmentBatchItem{original}}
	saved, err := send.ExecuteBatch(ctx, f.owner, input, domain.FileStorageBackendLocal)
	if err != nil {
		t.Fatal(err)
	}
	added := original
	added.ClientMessageID = uuid.NewV7().String()
	original.File.FileName = "changed.txt"
	input.Attachments = []conversationaction.AttachmentBatchItem{added, original}
	var conflict *conversationaction.ConflictError
	if _, err := send.ExecuteBatch(ctx, f.owner, input, domain.FileStorageBackendLocal); !errors.As(err, &conflict) {
		t.Fatalf("changed intent accepted: %v", err)
	}
	count, err := f.db.NewSelect().Model((*servermodels.File)(nil)).Where("organization_id = ?", f.owner.Organization.ID).Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("file rollback: %d %v", count, err)
	}
	count, err = f.db.NewSelect().Model((*servermodels.Message)(nil)).Where("conversation_id = ?", saved.ConversationID).Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("message rollback: %d %v", count, err)
	}
}

// TestAttachmentMessageReplies 验证附件引用的摘要、幂等、会话隔离及取消后的读取。
func TestAttachmentMessageReplies(t *testing.T) {
	for _, body := range []string{"", "附件的说明"} {
		t.Run("body="+body, func(t *testing.T) {
			f := newNavigationFixture(t)
			ctx := context.Background()
			attachments := conversationaction.NewSendAttachmentMessageAction(f.db)
			batch, err := attachments.ExecuteBatch(ctx, f.owner, conversationaction.AttachmentBatchInput{
				TargetIdentityID: f.member.OrganizationIdentity.ID,
				Attachments:      []conversationaction.AttachmentBatchItem{{ClientMessageID: uuid.NewV7().String(), Body: body, File: fileaction.UploadInput{FileName: "report.txt", ByteSize: 7}}},
			}, domain.FileStorageBackendLocal)
			if err != nil {
				t.Fatal(err)
			}
			target := batch.Messages[0]
			expected := body
			if expected == "" {
				expected = "report.txt"
			}
			send := conversationaction.NewSendDirectTextMessageAction(f.db)
			input := conversationaction.InternalTextMessageInput{ConversationID: batch.ConversationID, ClientMessageID: uuid.NewV7().String(), Body: "收到", ReplyToMessageID: target.ID}
			reply, err := send.Execute(ctx, f.member, input)
			if err != nil || reply.ReplyTo == nil || reply.ReplyTo.ID != target.ID || reply.ReplyTo.Type != domain.MessageTypeAttachment || reply.ReplyTo.Body != expected || reply.ReplyTo.Sender.SourceID != f.owner.OrganizationIdentity.ID {
				t.Fatalf("reply=%+v err=%v", reply, err)
			}
			replayed, err := send.Execute(ctx, f.member, input)
			if err != nil || replayed.ID != reply.ID || replayed.ReplyTo == nil || replayed.ReplyTo.Body != expected {
				t.Fatalf("replayed=%+v err=%v", replayed, err)
			}
			// 群聊不能引用同企业另一条单聊中的附件。
			_, err = conversationaction.NewSendGroupTextMessageAction(f.db).Execute(ctx, f.member, conversationaction.GroupTextMessageInput{ConversationID: f.groupID, ClientMessageID: uuid.NewV7().String(), Body: "跨会话引用", ReplyToMessageID: target.ID})
			var conflict *conversationaction.ConflictError
			if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonReplyTargetInvalid {
				t.Fatalf("cross-conversation reference=%v", err)
			}
			query := conversationaction.NewListConversationMessagesQuery(f.db)
			history, err := query.Execute(ctx, f.owner, conversationaction.ConversationMessageHistoryInput{ConversationID: batch.ConversationID})
			if err != nil || len(history.Messages) != 2 || history.Messages[1].ReplyTo == nil || history.Messages[1].ReplyTo.Body != expected || history.Messages[1].ReplyTo.Type != domain.MessageTypeAttachment {
				t.Fatalf("history=%+v err=%v", history, err)
			}
			if err := attachments.UpdateUploads(ctx, f.owner, []string{target.Attachment.ID}, domain.AttachmentCancelled); err != nil {
				t.Fatal(err)
			}
			history, err = query.Execute(ctx, f.member, conversationaction.ConversationMessageHistoryInput{ConversationID: batch.ConversationID})
			if err != nil || len(history.Messages) != 1 || history.Messages[0].ReplyTo == nil || !history.Messages[0].ReplyTo.Deleted || history.Messages[0].ReplyTo.Body != "" || history.Messages[0].ReplyTo.Sender != nil {
				t.Fatalf("cancelled history=%+v err=%v", history, err)
			}
			replayed, err = send.Execute(ctx, f.member, input)
			if err != nil || replayed.ID != reply.ID || replayed.ReplyTo == nil || !replayed.ReplyTo.Deleted {
				t.Fatalf("cancelled replay=%+v err=%v", replayed, err)
			}
			input.ClientMessageID = uuid.NewV7().String()
			if _, err := send.Execute(ctx, f.member, input); !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonReplyTargetInvalid {
				t.Fatalf("cancelled target accepted=%v", err)
			}
		})
	}
}
