-- +goose Up
-- 创建 AI 供应商模型表。
CREATE TABLE ai_provider_models (
    provider_id       uuid NOT NULL,
    organization_id   uuid NOT NULL,
    identifier        text NOT NULL,
    name              text NOT NULL,
    context_window    bigint NOT NULL,
    max_output_tokens bigint NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (provider_id, identifier)
);

COMMENT ON TABLE ai_provider_models IS 'AI 供应商已启用模型';
COMMENT ON COLUMN ai_provider_models.provider_id IS '供应商编号';
COMMENT ON COLUMN ai_provider_models.organization_id IS '所属企业编号';
COMMENT ON COLUMN ai_provider_models.identifier IS '模型标识';
COMMENT ON COLUMN ai_provider_models.name IS '模型名称';
COMMENT ON COLUMN ai_provider_models.context_window IS '上下文窗口 Token 数';
COMMENT ON COLUMN ai_provider_models.max_output_tokens IS '最大输出 Token 数';
COMMENT ON COLUMN ai_provider_models.created_at IS '添加时间';

CREATE INDEX ai_provider_models_organization_provider_index
    ON ai_provider_models (organization_id, provider_id);

-- +goose Down
DROP TABLE ai_provider_models;
