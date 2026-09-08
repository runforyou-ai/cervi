-- +goose Up
CREATE TABLE telegram_messages (
    message_id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    channel_id uuid NOT NULL,
    bot_id bigint NOT NULL,
    chat_id bigint NOT NULL,
    provider_message_id bigint NOT NULL,
    reply_provider_message_id bigint,
    reply_body text NOT NULL DEFAULT '',
    reply_sender_name text NOT NULL DEFAULT '',
    reply_sender_is_bot boolean NOT NULL DEFAULT false
);
COMMENT ON TABLE telegram_messages IS 'Telegram 消息身份与入站引用快照';
COMMENT ON COLUMN telegram_messages.message_id IS '会话消息编号';
COMMENT ON COLUMN telegram_messages.organization_id IS '所属企业';
COMMENT ON COLUMN telegram_messages.conversation_id IS '所属会话';
COMMENT ON COLUMN telegram_messages.channel_id IS '所属渠道';
COMMENT ON COLUMN telegram_messages.bot_id IS '机器人编号';
COMMENT ON COLUMN telegram_messages.chat_id IS '平台聊天编号';
COMMENT ON COLUMN telegram_messages.provider_message_id IS '平台消息编号';
COMMENT ON COLUMN telegram_messages.reply_provider_message_id IS '引用的平台消息编号';
COMMENT ON COLUMN telegram_messages.reply_body IS '引用原文快照';
COMMENT ON COLUMN telegram_messages.reply_sender_name IS '引用发送者名称快照';
COMMENT ON COLUMN telegram_messages.reply_sender_is_bot IS '引用发送者是否为机器人';
CREATE UNIQUE INDEX telegram_messages_provider_unique ON telegram_messages (organization_id, channel_id, bot_id, chat_id, provider_message_id);

-- +goose Down
DROP TABLE telegram_messages;
