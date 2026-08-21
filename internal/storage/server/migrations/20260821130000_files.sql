-- +goose Up
-- 创建企业文件元数据表，文件内容保存在本地目录或对象存储。
CREATE TABLE files (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
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
    expires_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX files_storage_key_unique
    ON files (storage_key);

CREATE INDEX files_organization_status_created_index
    ON files (organization_id, status, created_at DESC);

CREATE INDEX files_status_expires_index
    ON files (status, expires_at)
    WHERE expires_at IS NOT NULL;

-- +goose Down
DROP TABLE files;
