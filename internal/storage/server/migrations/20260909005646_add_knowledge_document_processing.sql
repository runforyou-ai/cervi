-- +goose Up
ALTER TABLE knowledge_documents
    ADD COLUMN processing_id uuid,
    ADD COLUMN segment_batch_id uuid,
    ADD COLUMN segment_count integer NOT NULL DEFAULT 0,
    ADD COLUMN failure_code varchar(64) NOT NULL DEFAULT '',
    ADD COLUMN chunk_length integer NOT NULL DEFAULT 512,
    ADD COLUMN chunk_overlap integer NOT NULL DEFAULT 50;
COMMENT ON COLUMN knowledge_documents.status IS '技术处理状态，产品展示状态由代码映射';
COMMENT ON COLUMN knowledge_documents.processing_id IS '当前处理任务的幂等标识';
COMMENT ON COLUMN knowledge_documents.segment_batch_id IS '已完成分段的批次编号';
COMMENT ON COLUMN knowledge_documents.segment_count IS '已完成分段的数量';
COMMENT ON COLUMN knowledge_documents.failure_code IS '处理失败原因码';
COMMENT ON COLUMN knowledge_documents.chunk_length IS '本次任务的分段长度，按字符计数';
COMMENT ON COLUMN knowledge_documents.chunk_overlap IS '本次任务的重叠长度，按字符计数';

-- +goose Down
ALTER TABLE knowledge_documents
    DROP COLUMN processing_id,
    DROP COLUMN segment_batch_id,
    DROP COLUMN segment_count,
    DROP COLUMN failure_code,
    DROP COLUMN chunk_length,
    DROP COLUMN chunk_overlap;
COMMENT ON COLUMN knowledge_documents.status IS '文档处理状态';
