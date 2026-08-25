-- +goose Up
-- 创建企业表。
CREATE TABLE organizations (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    name        text NOT NULL
);

COMMENT ON TABLE organizations IS '企业组织';
COMMENT ON COLUMN organizations.id IS '企业编号';
COMMENT ON COLUMN organizations.created_at IS '创建时间';
COMMENT ON COLUMN organizations.updated_at IS '更新时间';
COMMENT ON COLUMN organizations.name IS '企业名称';

-- +goose Down
DROP TABLE organizations;
