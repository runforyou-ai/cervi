-- +goose Up
ALTER TABLE knowledge_documents
    ADD COLUMN embedding_provider_id uuid,
    ADD COLUMN embedding_model_identifier text NOT NULL DEFAULT '',
    ADD COLUMN embedding_dimension integer NOT NULL DEFAULT 0;
COMMENT ON COLUMN knowledge_documents.embedding_provider_id IS '任务向量模型供应商编号';
COMMENT ON COLUMN knowledge_documents.embedding_model_identifier IS '任务向量模型标识';
COMMENT ON COLUMN knowledge_documents.embedding_dimension IS '任务向量维度';

-- +goose Down
ALTER TABLE knowledge_documents
    DROP COLUMN embedding_dimension,
    DROP COLUMN embedding_model_identifier,
    DROP COLUMN embedding_provider_id;
