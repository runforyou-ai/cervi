-- +goose Up
-- 创建企业表。
CREATE TABLE organizations (
    id                   uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    access_host          text NOT NULL,
    name                 text NOT NULL,
    allow_arbitrary_url  boolean NOT NULL DEFAULT false
);

COMMENT ON TABLE organizations IS '企业组织';
COMMENT ON COLUMN organizations.id IS '企业编号';
COMMENT ON COLUMN organizations.created_at IS '创建时间';
COMMENT ON COLUMN organizations.updated_at IS '更新时间';
COMMENT ON COLUMN organizations.access_host IS '企业规范化访问地址，包含非默认端口';
COMMENT ON COLUMN organizations.name IS '企业名称';
COMMENT ON COLUMN organizations.allow_arbitrary_url IS '是否允许成员打开任意网址';

CREATE UNIQUE INDEX organizations_access_host_unique
    ON organizations (access_host);

-- +goose Down
DROP TABLE organizations;
