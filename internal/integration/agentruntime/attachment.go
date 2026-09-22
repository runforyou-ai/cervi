package agentruntime

import (
	"context"
	"encoding/base64"
	"log/slog"
	"sync/atomic"

	"github.com/cloudwego/eino/components/model"
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

// mediaTrackingModel 记录携带直传附件的模型调用是否失败，运行据此改用正文链接重新执行。
type mediaTrackingModel struct {
	model.AgenticModel
	rejected *atomic.Bool
}

// Generate 调用模型，携带直传附件的请求失败时记录拒绝状态。
func (m *mediaTrackingModel) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	output, err := m.AgenticModel.Generate(ctx, input, opts...)
	if err != nil && carriesMedia(input) {
		m.rejected.Store(true)
	}
	return output, err
}

// Stream 以流式调用模型，携带直传附件的请求在建立流或读取分片时失败都记录拒绝状态。
func (m *mediaTrackingModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	output, err := m.AgenticModel.Stream(ctx, input, opts...)
	if !carriesMedia(input) {
		return output, err
	}
	if err != nil {
		m.rejected.Store(true)
		return nil, err
	}
	return schema.StreamReaderWithConvert(output, func(chunk *schema.AgenticMessage) (*schema.AgenticMessage, error) {
		return chunk, nil
	}, schema.WithErrWrapper(func(err error) error {
		m.rejected.Store(true)
		return err
	})), nil
}

// carriesMedia 判断模型输入是否包含直传的图片、音频或视频内容块。
func carriesMedia(input []*schema.AgenticMessage) bool {
	for _, message := range input {
		for _, block := range message.ContentBlocks {
			switch block.Type {
			case schema.ContentBlockTypeUserInputImage, schema.ContentBlockTypeUserInputAudio, schema.ContentBlockTypeUserInputVideo:
				return true
			}
		}
	}
	return false
}

// mediaUserMessage 读取附件并构造正文块与多模态块并列的用户消息，读取失败或模态不可直传时返回 false。
func mediaUserMessage(ctx context.Context, message Message, modality domain.AIModelInputModality, read AttachmentContent) (*schema.AgenticMessage, bool) {
	content, err := read(ctx, message.ID)
	if err != nil {
		slog.Warn("读取直传附件失败，仅以正文链接提供给模型",
			"agent_run_id", runIDFromContext(ctx), "message_id", message.ID, "error", err)
		return nil, false
	}
	data := base64.StdEncoding.EncodeToString(content)
	var media *schema.ContentBlock
	switch modality {
	case domain.AIModelInputModalityImage:
		media = schema.NewContentBlock(&schema.UserInputImage{Base64Data: data, MIMEType: message.Media.MIMEType})
	case domain.AIModelInputModalityAudio:
		media = schema.NewContentBlock(&schema.UserInputAudio{Base64Data: data, MIMEType: message.Media.MIMEType})
	case domain.AIModelInputModalityVideo:
		media = schema.NewContentBlock(&schema.UserInputVideo{Base64Data: data, MIMEType: message.Media.MIMEType})
	default:
		return nil, false
	}
	return &schema.AgenticMessage{Role: schema.AgenticRoleTypeUser, ContentBlocks: []*schema.ContentBlock{
		schema.NewContentBlock(&schema.UserInputText{Text: message.Content}),
		media,
	}}, true
}
