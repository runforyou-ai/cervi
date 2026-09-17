package domain

import "strings"

// MessageAttachmentTransferStatus 表示附件内容的取回状态。
type MessageAttachmentTransferStatus string

const (
	MessageAttachmentTransferReady   MessageAttachmentTransferStatus = "ready"
	MessageAttachmentTransferPending MessageAttachmentTransferStatus = "pending"
	MessageAttachmentTransferFailed  MessageAttachmentTransferStatus = "failed"
)

// AttachmentIsPhoto 判断附件内容类型是否为外部渠道按照片投递的图片。
func AttachmentIsPhoto(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(contentType), "image/")
}
