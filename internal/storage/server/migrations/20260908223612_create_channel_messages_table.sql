-- +goose Up
CREATE TABLE channel_messages (
    message_id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    channel_id uuid NOT NULL,
    provider_account_id text NOT NULL,
    provider_conversation_id text NOT NULL,
    provider_message_id text NOT NULL,
    reply_provider_message_id text,
    reply_body text NOT NULL DEFAULT '',
    reply_sender_name text NOT NULL DEFAULT '',
    reply_sender_is_bot boolean NOT NULL DEFAULT false
);
COMMENT ON TABLE channel_messages IS '渠道消息身份与入站引用快照';
COMMENT ON COLUMN channel_messages.message_id IS '会话消息编号';
COMMENT ON COLUMN channel_messages.organization_id IS '所属企业';
COMMENT ON COLUMN channel_messages.conversation_id IS '所属会话';
COMMENT ON COLUMN channel_messages.channel_id IS '所属渠道';
COMMENT ON COLUMN channel_messages.provider_account_id IS '平台账号编号';
COMMENT ON COLUMN channel_messages.provider_conversation_id IS '平台聊天编号';
COMMENT ON COLUMN channel_messages.provider_message_id IS '平台消息编号';
COMMENT ON COLUMN channel_messages.reply_provider_message_id IS '引用的平台消息编号';
COMMENT ON COLUMN channel_messages.reply_body IS '引用原文快照';
COMMENT ON COLUMN channel_messages.reply_sender_name IS '引用发送者名称快照';
COMMENT ON COLUMN channel_messages.reply_sender_is_bot IS '引用发送者是否为机器人';
CREATE UNIQUE INDEX channel_messages_provider_unique ON channel_messages (organization_id, channel_id, provider_account_id, provider_conversation_id, provider_message_id);

-- +goose Down
DROP TABLE channel_messages;
