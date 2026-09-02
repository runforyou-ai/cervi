-- +goose Up
ALTER TABLE conversations
    ADD COLUMN description text,
    ADD COLUMN image_file_id uuid;

COMMENT ON COLUMN conversations.description IS '群聊描述';
COMMENT ON COLUMN conversations.image_file_id IS '群聊图片文件编号';

-- +goose Down
ALTER TABLE conversations
    DROP COLUMN image_file_id,
    DROP COLUMN description;
