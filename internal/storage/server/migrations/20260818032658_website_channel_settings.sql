-- +goose Up
-- 创建网站渠道设置表，关联关系由 Action 维护。
CREATE TABLE website_channel_settings (
    channel_id          uuid PRIMARY KEY,
    organization_id     uuid NOT NULL,
    chat_title          text NOT NULL,
    chat_subtitle       text,
    greeting_message    text,
    theme_color         text NOT NULL DEFAULT '#2563EB',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE website_channel_settings IS '网站渠道聊天界面设置';
COMMENT ON COLUMN website_channel_settings.channel_id IS '渠道编号';
COMMENT ON COLUMN website_channel_settings.organization_id IS '所属企业编号';
COMMENT ON COLUMN website_channel_settings.chat_title IS '聊天窗口标题';
COMMENT ON COLUMN website_channel_settings.chat_subtitle IS '聊天窗口副标题';
COMMENT ON COLUMN website_channel_settings.greeting_message IS '访客欢迎语';
COMMENT ON COLUMN website_channel_settings.theme_color IS '界面主题色';
COMMENT ON COLUMN website_channel_settings.created_at IS '创建时间';
COMMENT ON COLUMN website_channel_settings.updated_at IS '更新时间';

CREATE INDEX website_channel_settings_organization_channel_index
    ON website_channel_settings (organization_id, channel_id);

-- +goose Down
DROP TABLE website_channel_settings;
