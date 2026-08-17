-- +goose Up
CREATE TABLE website_channel_settings (
    channel_id          uuid PRIMARY KEY,
    organization_id     uuid NOT NULL,
    chat_title          text NOT NULL
                            CHECK (
                                btrim(chat_title) <> ''
                                AND char_length(chat_title) <= 100
                            ),
    chat_subtitle       text
                            CHECK (
                                chat_subtitle IS NULL
                                OR char_length(chat_subtitle) <= 120
                            ),
    greeting_message    text
                            CHECK (
                                greeting_message IS NULL
                                OR char_length(greeting_message) <= 500
                            ),
    theme_color         text NOT NULL DEFAULT '#2563EB'
                            CHECK (theme_color ~ '^#[0-9A-F]{6}$'),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX website_channel_settings_organization_channel_index
    ON website_channel_settings (organization_id, channel_id);

-- +goose Down
DROP TABLE website_channel_settings;
