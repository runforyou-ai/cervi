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

CREATE INDEX website_channel_settings_organization_channel_index
    ON website_channel_settings (organization_id, channel_id);

-- +goose Down
DROP TABLE website_channel_settings;
