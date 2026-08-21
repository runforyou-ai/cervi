-- +goose Up
-- 创建企业团队表，关联关系由 Action 维护。
CREATE TABLE teams (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id     uuid NOT NULL,
    name                text NOT NULL,
    description         text NOT NULL DEFAULT '',
    created_by_user_id  uuid NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX teams_organization_name_unique
    ON teams (organization_id, lower(name));

-- +goose Down
DROP TABLE teams;
