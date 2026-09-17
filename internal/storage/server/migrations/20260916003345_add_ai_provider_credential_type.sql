-- +goose Up
-- 为模型服务供应商增加凭据类型，并放开无凭据服务的密钥约束。
ALTER TABLE ai_providers ADD COLUMN credential_type text NOT NULL DEFAULT 'api_key';
ALTER TABLE ai_providers ALTER COLUMN credential_type DROP DEFAULT;

COMMENT ON COLUMN ai_providers.credential_type IS '凭据类型：api_key 使用密钥，none 不需要凭据';
COMMENT ON COLUMN ai_providers.api_key IS 'API 密钥，凭据类型为 none 时为空';

-- +goose Down
ALTER TABLE ai_providers DROP COLUMN credential_type;
COMMENT ON COLUMN ai_providers.api_key IS 'API 密钥';
