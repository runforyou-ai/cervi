//go:build server

package agentruntime

import (
	"context"
	"encoding/base64"
	"log/slog"

	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const (
	// mediaWindowPercent 随消息直传的附件最多占模型窗口的百分比，超出预算的较早附件只保留正文中的链接。
	mediaWindowPercent = 20
	// mediaTokens 按常见多模态模型单个图片或媒体片段的输入上限估算 Token。
	mediaTokens = 1280
	// maxMediaBytes 单个附件随请求直传的字节上限，超出时只保留正文中的链接。
	maxMediaBytes = 10 << 20
	// maxRunMediaBytes 单次运行上下文直传附件的累计字节上限。
	maxRunMediaBytes = 20 << 20
)

// inlineMediaTypes 按输入模态列出可随请求直传的附件格式；音频限于模型组件能识别格式的 wav。
var inlineMediaTypes = map[string]domain.AIModelInputModality{
	"image/png":       domain.AIModelInputModalityImage,
	"image/jpeg":      domain.AIModelInputModalityImage,
	"image/webp":      domain.AIModelInputModalityImage,
	"image/gif":       domain.AIModelInputModalityImage,
	"audio/wav":       domain.AIModelInputModalityAudio,
	"audio/wave":      domain.AIModelInputModalityAudio,
	"audio/vnd.wav":   domain.AIModelInputModalityAudio,
	"audio/vnd.wave":  domain.AIModelInputModalityAudio,
	"audio/x-pn-wav":  domain.AIModelInputModalityAudio,
	"video/mp4":       domain.AIModelInputModalityVideo,
	"video/webm":      domain.AIModelInputModalityVideo,
	"video/quicktime": domain.AIModelInputModalityVideo,
}

// mediaInput 定义本次运行读取附件的方式、模型接受的输入模态和直传附件数量上限。
type mediaInput struct {
	read       AttachmentContent
	modalities map[domain.AIModelInputModality]bool
	maxCount   int
}

// mediaUserMessage 读取附件并构造正文与多模态内容并列的用户消息，读取失败或模态不可直传时返回 false。
func mediaUserMessage(ctx context.Context, message Message, modality domain.AIModelInputModality, read AttachmentContent) (*schema.Message, bool) {
	content, err := read(ctx, message.ID)
	if err != nil {
		slog.Warn("读取直传附件失败，仅以正文链接提供给模型",
			"agent_run_id", runIDFromContext(ctx), "message_id", message.ID, "error", err)
		return nil, false
	}
	data := base64.StdEncoding.EncodeToString(content)
	common := schema.MessagePartCommon{Base64Data: &data, MIMEType: message.Media.MIMEType}
	var part schema.MessageInputPart
	switch modality {
	case domain.AIModelInputModalityImage:
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common}}
	case domain.AIModelInputModalityAudio:
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: common}}
	case domain.AIModelInputModalityVideo:
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: common}}
	default:
		return nil, false
	}
	return &schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: message.Content},
		part,
	}}, true
}
