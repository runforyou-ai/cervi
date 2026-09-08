-- +goose Up
ALTER TABLE files ADD COLUMN multipart_upload_id text;
ALTER TABLE files ADD COLUMN part_size bigint NOT NULL DEFAULT 0;
COMMENT ON COLUMN files.multipart_upload_id IS '对象存储分片上传会话编号';
COMMENT ON COLUMN files.part_size IS '分片字节数，0 表示整体上传';
COMMENT ON COLUMN files.purpose IS '文件用途：user_avatar 用户头像、contact_avatar 联系人头像、group_image 群图片、message_attachment 消息附件';

-- +goose Down
ALTER TABLE files DROP COLUMN multipart_upload_id;
ALTER TABLE files DROP COLUMN part_size;
COMMENT ON COLUMN files.purpose IS '文件业务用途';
