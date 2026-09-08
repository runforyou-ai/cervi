-- +goose Up
CREATE TABLE message_batches (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    sender_identity_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    request_digest text NOT NULL,
    message_ids uuid[] NOT NULL
);
COMMENT ON TABLE message_batches IS '一次发送的有序消息及幂等意图';
COMMENT ON COLUMN message_batches.id IS '客户端批次编号';
COMMENT ON COLUMN message_batches.organization_id IS '所属企业编号';
COMMENT ON COLUMN message_batches.sender_identity_id IS '发送者身份编号';
COMMENT ON COLUMN message_batches.conversation_id IS '会话编号';
COMMENT ON COLUMN message_batches.request_digest IS '原始发送意图摘要';
COMMENT ON COLUMN message_batches.message_ids IS '按发送顺序保存的消息编号';
-- +goose Down
DROP TABLE message_batches;
