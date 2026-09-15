-- +goose Up
ALTER TABLE message_attachments
    DROP COLUMN upload_status,
    DROP COLUMN upload_expires_at;

-- +goose Down
ALTER TABLE message_attachments
    ADD COLUMN upload_status text NOT NULL DEFAULT 'ready',
    ADD COLUMN upload_expires_at timestamptz;
ALTER TABLE message_attachments ALTER COLUMN upload_status DROP DEFAULT;
COMMENT ON COLUMN message_attachments.upload_status IS '上传状态：uploading、ready、failed、cancelled';
COMMENT ON COLUMN message_attachments.upload_expires_at IS '上传活跃期限';
