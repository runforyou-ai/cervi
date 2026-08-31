-- +goose Up
-- 创建企业 AI 供应商表。
CREATE TABLE ai_providers (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    organization_id  uuid NOT NULL,
    brand            text NOT NULL,
    name             text NOT NULL,
    api_key          text NOT NULL,
    api_url          text NOT NULL
);

COMMENT ON TABLE ai_providers IS '企业 AI 供应商';
COMMENT ON COLUMN ai_providers.id IS '供应商编号';
COMMENT ON COLUMN ai_providers.created_at IS '添加时间';
COMMENT ON COLUMN ai_providers.updated_at IS '更新时间';
COMMENT ON COLUMN ai_providers.organization_id IS '所属企业编号';
COMMENT ON COLUMN ai_providers.brand IS '供应商品牌';
COMMENT ON COLUMN ai_providers.name IS '供应商名称';
COMMENT ON COLUMN ai_providers.api_key IS 'API 密钥';
COMMENT ON COLUMN ai_providers.api_url IS 'API 地址';

CREATE UNIQUE INDEX ai_providers_organization_name_unique
    ON ai_providers (organization_id, lower(name));

CREATE INDEX ai_providers_organization_created_at_index
    ON ai_providers (organization_id, created_at);

-- +goose Down
DROP TABLE ai_providers;
