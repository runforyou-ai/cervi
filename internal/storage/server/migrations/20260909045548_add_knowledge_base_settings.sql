-- +goose Up
ALTER TABLE knowledge_bases
    ADD COLUMN embedding_provider_id uuid,
    ADD COLUMN embedding_model_identifier text,
    ADD COLUMN embedding_dimension bigint,
    ADD COLUMN chunk_length integer,
    ADD COLUMN chunk_overlap integer,
    ADD COLUMN retrieval_count integer,
    ADD COLUMN rerank_provider_id uuid,
    ADD COLUMN rerank_model_identifier text;

COMMENT ON COLUMN knowledge_bases.embedding_provider_id IS '向量模型供应商编号';
COMMENT ON COLUMN knowledge_bases.embedding_model_identifier IS '向量模型标识';
COMMENT ON COLUMN knowledge_bases.embedding_dimension IS '向量维度';
COMMENT ON COLUMN knowledge_bases.chunk_length IS '分段长度，问答库不设置';
COMMENT ON COLUMN knowledge_bases.chunk_overlap IS '分段重叠，问答库不设置';
COMMENT ON COLUMN knowledge_bases.retrieval_count IS '召回数量';
COMMENT ON COLUMN knowledge_bases.rerank_provider_id IS '重排模型供应商编号，空值表示关闭重排';
COMMENT ON COLUMN knowledge_bases.rerank_model_identifier IS '重排模型标识';

-- +goose Down
ALTER TABLE knowledge_bases
    DROP COLUMN rerank_model_identifier,
    DROP COLUMN rerank_provider_id,
    DROP COLUMN retrieval_count,
    DROP COLUMN chunk_overlap,
    DROP COLUMN chunk_length,
    DROP COLUMN embedding_dimension,
    DROP COLUMN embedding_model_identifier,
    DROP COLUMN embedding_provider_id;
