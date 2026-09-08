//go:build server

package models

import "github.com/uptrace/bun"

// MessageBatch 保存一次有序发送的完整幂等意图和结果。
type MessageBatch struct {
	bun.BaseModel    `bun:"table:message_batches,alias:mb"`
	ID               string   `bun:"id,pk"`
	OrganizationID   string   `bun:"organization_id"`
	SenderIdentityID string   `bun:"sender_identity_id"`
	ConversationID   string   `bun:"conversation_id"`
	RequestDigest    string   `bun:"request_digest"`
	MessageIDs       []string `bun:"message_ids,array"`
}
