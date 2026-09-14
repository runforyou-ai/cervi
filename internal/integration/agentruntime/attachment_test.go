//go:build server

package agentruntime

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestTurnHistoryInlinesRecentMedia 验证附件按模型模态由新到旧在预算内直传，其余只保留正文。
func TestTurnHistoryInlinesRecentMedia(t *testing.T) {
	read := func(_ context.Context, messageID string) ([]byte, error) {
		if messageID == "broken" {
			return nil, errors.New("storage unavailable")
		}
		return []byte("media:" + messageID), nil
	}
	png := &Media{MIMEType: "image/png", ByteSize: 1024}
	visual := map[domain.AIModelInputModality]bool{domain.AIModelInputModalityImage: true, domain.AIModelInputModalityAudio: true}
	history := &turnHistory{}
	got := history.appendInput(context.Background(), []Message{
		{ID: "old", Role: MessageRoleUser, Content: "旧图", Media: png},
		{ID: "svg", Role: MessageRoleUser, Content: "矢量图", Media: &Media{MIMEType: "image/svg+xml", ByteSize: 10}},
		{ID: "video", Role: MessageRoleUser, Content: "视频", Media: &Media{MIMEType: "video/mp4", ByteSize: 10}},
		{ID: "large", Role: MessageRoleUser, Content: "大图", Media: &Media{MIMEType: "image/png", ByteSize: maxMediaBytes + 1}},
		{ID: "voice", Role: MessageRoleUser, Content: "语音", Media: &Media{MIMEType: "audio/wav", ByteSize: 2048}},
		{ID: "new", Role: MessageRoleUser, Content: "新图", Media: png},
	}, mediaInput{read: read, modalities: visual, maxCount: 2})
	if len(got) != 6 {
		t.Fatalf("history = %#v", got)
	}
	for _, index := range []int{0, 1, 2, 3} {
		if len(got[index].UserInputMultiContent) != 0 || got[index].Content == "" {
			t.Fatalf("message %d inlined unexpectedly: %#v", index, got[index])
		}
	}
	voice := got[4].UserInputMultiContent
	if len(voice) != 2 || voice[0].Text != "语音" || voice[1].Audio == nil || voice[1].Audio.MIMEType != "audio/wav" {
		t.Fatalf("audio message = %#v", got[4])
	}
	parts := got[5].UserInputMultiContent
	if got[5].Content != "" || len(parts) != 2 || parts[0].Text != "新图" || parts[1].Image == nil ||
		parts[1].Image.MIMEType != "image/png" || *parts[1].Image.Base64Data != base64.StdEncoding.EncodeToString([]byte("media:new")) {
		t.Fatalf("latest image message = %#v", got[5])
	}
	// 后续轮次补入的附件沿用本次运行已占用的数量预算。
	next := history.appendInput(context.Background(), []Message{{ID: "later", Role: MessageRoleUser, Content: "再一张", Media: png}}, mediaInput{read: read, modalities: visual, maxCount: 2})
	if len(next[6].UserInputMultiContent) != 0 {
		t.Fatalf("media beyond run budget inlined: %#v", next[6])
	}
	failed := (&turnHistory{}).appendInput(context.Background(), []Message{
		{ID: "broken", Role: MessageRoleUser, Content: "读取失败", Media: png},
		{ID: "reply", Role: MessageRoleAssistant, Content: "助手消息", Media: png},
	}, mediaInput{read: read, modalities: visual, maxCount: 5})
	if failed[0].Content != "读取失败" || len(failed[0].UserInputMultiContent) != 0 || failed[1].Role != schema.Assistant || len(failed[1].UserInputMultiContent) != 0 {
		t.Fatalf("fallback history = %#v", failed)
	}
}

// TestCountContextTokensIncludesMediaParts 验证多模态消息的正文片段和媒体片段都计入上下文估算。
func TestCountContextTokensIncludesMediaParts(t *testing.T) {
	data := "bWVkaWE="
	common := schema.MessagePartCommon{Base64Data: &data, MIMEType: "image/png"}
	message := &schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "看图"},
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common}},
		{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: common}},
	}}
	tokens, err := countContextTokens(context.Background(), []*schema.Message{message}, nil)
	if err != nil || tokens != 2*mediaTokens+2 {
		t.Fatalf("tokens = %d, err = %v", tokens, err)
	}
}
