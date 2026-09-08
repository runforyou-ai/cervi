-- +goose Up
CREATE TABLE message_attachments (
    message_id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    file_id uuid UNIQUE,
    name text NOT NULL,
    content_type text NOT NULL,
    byte_size bigint NOT NULL,
    image_width integer NOT NULL DEFAULT 0,
    image_height integer NOT NULL DEFAULT 0,
    upload_status text NOT NULL,
    upload_expires_at timestamptz
);
COMMENT ON TABLE message_attachments IS '附件消息关联的文件';
COMMENT ON COLUMN message_attachments.message_id IS '消息编号';
COMMENT ON COLUMN message_attachments.organization_id IS '所属企业编号';
COMMENT ON COLUMN message_attachments.file_id IS '附件文件编号，过期清理后为空';
COMMENT ON COLUMN message_attachments.name IS '原始文件名';
COMMENT ON COLUMN message_attachments.content_type IS '内容类型';
COMMENT ON COLUMN message_attachments.byte_size IS '文件字节数';
COMMENT ON COLUMN message_attachments.image_width IS '图片宽度，非图片为 0';
COMMENT ON COLUMN message_attachments.image_height IS '图片高度，非图片为 0';
COMMENT ON COLUMN message_attachments.upload_status IS '上传状态：uploading、ready、failed、cancelled';
COMMENT ON COLUMN message_attachments.upload_expires_at IS '上传活跃期限';
COMMENT ON INDEX message_attachments_file_id_key IS '一个上传文件仅关联一条消息';

-- +goose Down
DROP TABLE message_attachments;
