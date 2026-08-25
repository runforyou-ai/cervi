-- +goose Up
-- 创建登录令牌表。
CREATE TABLE tokens (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at  timestamptz NOT NULL DEFAULT now(),
    user_id     uuid NOT NULL,
    token_hash  text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL
);

COMMENT ON TABLE tokens IS '用户登录令牌';
COMMENT ON COLUMN tokens.id IS '令牌编号';
COMMENT ON COLUMN tokens.created_at IS '创建时间';
COMMENT ON COLUMN tokens.user_id IS '用户编号';
COMMENT ON COLUMN tokens.token_hash IS '令牌哈希';
COMMENT ON COLUMN tokens.expires_at IS '过期时间';

CREATE INDEX tokens_user_id_index ON tokens (user_id);
CREATE INDEX tokens_expires_at_index ON tokens (expires_at);

-- +goose Down
DROP TABLE tokens;
