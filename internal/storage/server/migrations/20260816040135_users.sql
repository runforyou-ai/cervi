-- +goose Up
-- 创建企业成员表，关联关系由 Action 维护。
CREATE TABLE users (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL,
    email            text NOT NULL CHECK (btrim(email) <> ''),
    display_name     text NOT NULL CHECK (btrim(display_name) <> ''),
    password_hash    text NOT NULL,
    role             text NOT NULL DEFAULT 'member',
    status           text NOT NULL DEFAULT 'active',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_organization_email_unique
    ON users (organization_id, lower(email));

-- +goose Down
DROP TABLE users;
