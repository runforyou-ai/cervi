-- +goose Up
-- 创建 Telegram 渠道连接设置表。
CREATE TABLE telegram_channel_settings (
    channel_id            uuid PRIMARY KEY,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    organization_id       uuid NOT NULL,
    bot_token             text,
    bot_id                bigint,
    bot_username          text,
    bot_display_name      text,
    webhook_base_url      text,
    webhook_secret        text,
    webhook_status        text,
    webhook_connected_at  timestamptz
);

COMMENT ON TABLE telegram_channel_settings IS 'Telegram 渠道连接设置';
COMMENT ON COLUMN telegram_channel_settings.channel_id IS '渠道编号';
COMMENT ON COLUMN telegram_channel_settings.created_at IS '创建时间';
COMMENT ON COLUMN telegram_channel_settings.updated_at IS '更新时间';
COMMENT ON COLUMN telegram_channel_settings.organization_id IS '所属企业编号';
COMMENT ON COLUMN telegram_channel_settings.bot_token IS '机器人访问令牌';
COMMENT ON COLUMN telegram_channel_settings.bot_id IS 'Telegram 机器人编号';
COMMENT ON COLUMN telegram_channel_settings.bot_username IS 'Telegram 机器人用户名';
COMMENT ON COLUMN telegram_channel_settings.bot_display_name IS 'Telegram 机器人显示名称';
COMMENT ON COLUMN telegram_channel_settings.webhook_base_url IS 'Webhook 企业服务器基础地址';
COMMENT ON COLUMN telegram_channel_settings.webhook_secret IS 'Webhook 当前注册密钥';
COMMENT ON COLUMN telegram_channel_settings.webhook_status IS 'Webhook 连接状态';
COMMENT ON COLUMN telegram_channel_settings.webhook_connected_at IS 'Webhook 最近连接时间';

CREATE INDEX telegram_channel_settings_organization_channel_index
    ON telegram_channel_settings (organization_id, channel_id);

CREATE INDEX telegram_channel_settings_bot_id_index
    ON telegram_channel_settings (bot_id)
    WHERE bot_id IS NOT NULL;

-- +goose Down
DROP TABLE telegram_channel_settings;
