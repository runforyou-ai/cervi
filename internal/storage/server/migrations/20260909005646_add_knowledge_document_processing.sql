-- +goose Up
ALTER TABLE knowledge_documents
    ADD COLUMN processing_id uuid,
    ADD COLUMN segment_batch_id uuid,
    ADD COLUMN segment_count integer NOT NULL DEFAULT 0,
    ADD COLUMN failure_code varchar(64) NOT NULL DEFAULT '',
    ADD COLUMN chunk_length integer NOT NULL DEFAULT 512,
    ADD COLUMN chunk_overlap integer NOT NULL DEFAULT 50;
COMMENT ON COLUMN knowledge_documents.status IS '文档处理状态';
COMMENT ON COLUMN knowledge_documents.processing_id IS '当前处理任务的幂等标识';
COMMENT ON COLUMN knowledge_documents.segment_batch_id IS '已发布分段批次编号';
COMMENT ON COLUMN knowledge_documents.segment_count IS '已发布分段数量';
COMMENT ON COLUMN knowledge_documents.failure_code IS '处理失败原因码';
COMMENT ON COLUMN knowledge_documents.chunk_length IS '任务分段长度（字符数）';
COMMENT ON COLUMN knowledge_documents.chunk_overlap IS '任务分段重叠长度（字符数）';

-- +goose Down
ALTER TABLE knowledge_documents
    DROP COLUMN processing_id,
    DROP COLUMN segment_batch_id,
    DROP COLUMN segment_count,
    DROP COLUMN failure_code,
    DROP COLUMN chunk_length,
    DROP COLUMN chunk_overlap;
COMMENT ON COLUMN knowledge_documents.status IS '文档处理状态';
