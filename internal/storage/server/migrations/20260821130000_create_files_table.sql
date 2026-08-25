-- +goose Up
-- 创建企业文件元数据表。
CREATE TABLE files (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    organization_id     uuid NOT NULL,
    created_by_user_id  uuid NOT NULL,
    purpose             text NOT NULL,
    storage_backend     text NOT NULL,
    storage_key         text NOT NULL,
    original_name       text NOT NULL,
    content_type        text NOT NULL,
    byte_size           bigint NOT NULL,
    status              text NOT NULL DEFAULT 'pending',
    etag                text,
    uploaded_at         timestamptz,
    expires_at          timestamptz
);

COMMENT ON TABLE files IS '企业上传文件元数据';
COMMENT ON COLUMN files.id IS '文件编号';
COMMENT ON COLUMN files.created_at IS '创建时间';
COMMENT ON COLUMN files.updated_at IS '更新时间';
COMMENT ON COLUMN files.organization_id IS '所属企业编号';
COMMENT ON COLUMN files.created_by_user_id IS '上传用户编号';
COMMENT ON COLUMN files.purpose IS '文件业务用途';
COMMENT ON COLUMN files.storage_backend IS '本地或对象存储类型';
COMMENT ON COLUMN files.storage_key IS '文件在存储中的对象键';
COMMENT ON COLUMN files.original_name IS '用户上传的原始文件名';
COMMENT ON COLUMN files.content_type IS 'MIME 类型';
COMMENT ON COLUMN files.byte_size IS '文件字节数';
COMMENT ON COLUMN files.status IS '文件生命周期状态';
COMMENT ON COLUMN files.etag IS '对象存储 ETag';
COMMENT ON COLUMN files.uploaded_at IS '上传完成时间';
COMMENT ON COLUMN files.expires_at IS '临时文件过期或删除任务执行时间';

CREATE UNIQUE INDEX files_storage_key_unique
    ON files (storage_key);

CREATE INDEX files_organization_status_created_index
    ON files (organization_id, status, created_at DESC);

CREATE INDEX files_status_expires_index
    ON files (status, expires_at)
    WHERE expires_at IS NOT NULL;

-- +goose Down
DROP TABLE files;
