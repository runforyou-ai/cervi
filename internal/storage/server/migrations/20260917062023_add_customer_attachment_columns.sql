-- +goose Up
ALTER TABLE message_attachments
    ADD COLUMN transfer_status text NOT NULL DEFAULT 'ready';
ALTER TABLE message_attachments ALTER COLUMN transfer_status DROP DEFAULT;
COMMENT ON COLUMN message_attachments.transfer_status IS '附件内容取回状态：ready 已就绪、pending 取回中、failed 取回失败';

ALTER TABLE files
    ADD COLUMN uploader_channel_identity_id uuid;
COMMENT ON COLUMN files.uploader_channel_identity_id IS '上传该文件的渠道访客身份编号，成员上传为空';

-- +goose Down
ALTER TABLE files
    DROP COLUMN uploader_channel_identity_id;
ALTER TABLE message_attachments
    DROP COLUMN transfer_status;
