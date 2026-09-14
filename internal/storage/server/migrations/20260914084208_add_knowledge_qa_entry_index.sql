-- +goose Up
ALTER TABLE knowledge_qa_entries
    ADD COLUMN status varchar(32) NOT NULL DEFAULT 'initial',
    ADD COLUMN processing_id uuid,
    ADD COLUMN segment_batch_id uuid,
    ADD COLUMN segment_count integer NOT NULL DEFAULT 0,
    ADD COLUMN failure_code varchar(64) NOT NULL DEFAULT '',
    ADD COLUMN embedding_provider_id uuid,
    ADD COLUMN embedding_model_identifier text NOT NULL DEFAULT '',
    ADD COLUMN embedding_dimension integer NOT NULL DEFAULT 0;
COMMENT ON COLUMN knowledge_qa_entries.status IS '问答索引状态';
COMMENT ON COLUMN knowledge_qa_entries.processing_id IS '当前索引任务的幂等标识';
COMMENT ON COLUMN knowledge_qa_entries.segment_batch_id IS '已发布分段批次编号';
COMMENT ON COLUMN knowledge_qa_entries.segment_count IS '已发布分段数量';
COMMENT ON COLUMN knowledge_qa_entries.failure_code IS '索引失败原因码';
COMMENT ON COLUMN knowledge_qa_entries.embedding_provider_id IS '任务向量模型供应商编号';
COMMENT ON COLUMN knowledge_qa_entries.embedding_model_identifier IS '任务向量模型标识';
COMMENT ON COLUMN knowledge_qa_entries.embedding_dimension IS '任务向量维度';

-- +goose Down
ALTER TABLE knowledge_qa_entries
    DROP COLUMN embedding_dimension,
    DROP COLUMN embedding_model_identifier,
    DROP COLUMN embedding_provider_id,
    DROP COLUMN failure_code,
    DROP COLUMN segment_count,
    DROP COLUMN segment_batch_id,
    DROP COLUMN processing_id,
    DROP COLUMN status;
