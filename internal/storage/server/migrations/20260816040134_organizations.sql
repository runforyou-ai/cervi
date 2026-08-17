-- +goose Up
-- 创建企业组织表。
CREATE TABLE organizations (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    name        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE organizations;
