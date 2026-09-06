-- +goose Up
ALTER TABLE messages ADD COLUMN body_format VARCHAR(16) NOT NULL DEFAULT 'plain';
COMMENT ON COLUMN messages.body_format IS '消息正文格式：纯文本或 Markdown';

-- +goose Down
ALTER TABLE messages DROP COLUMN body_format;
