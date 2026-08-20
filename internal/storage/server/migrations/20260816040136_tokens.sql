-- +goose Up
-- 创建登录令牌表，关联关系由 Action 维护。
CREATE TABLE tokens (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id     uuid NOT NULL,
    token_hash  text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX tokens_user_id_index ON tokens (user_id);
CREATE INDEX tokens_expires_at_index ON tokens (expires_at);

-- +goose Down
DROP TABLE tokens;
