-- +goose Up
ALTER TABLE knowledge_bases
    ALTER COLUMN rerank_provider_id SET NOT NULL,
    ALTER COLUMN rerank_model_identifier SET NOT NULL;
COMMENT ON COLUMN knowledge_bases.rerank_provider_id IS '重排模型供应商编号';

-- +goose Down
ALTER TABLE knowledge_bases
    ALTER COLUMN rerank_provider_id DROP NOT NULL,
    ALTER COLUMN rerank_model_identifier DROP NOT NULL;
COMMENT ON COLUMN knowledge_bases.rerank_provider_id IS '重排模型供应商编号，空值表示关闭重排';
