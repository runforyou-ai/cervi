-- +goose Up
DROP INDEX public.knowledge_segments_batch_position_unique;
ALTER TABLE public.knowledge_segments RENAME COLUMN document_id TO source_id;
ALTER TABLE public.knowledge_segments ADD COLUMN source_type varchar(32) NOT NULL DEFAULT 'document';
ALTER TABLE public.knowledge_segments ALTER COLUMN source_type DROP DEFAULT;
CREATE UNIQUE INDEX knowledge_segments_batch_position_unique ON public.knowledge_segments (source_id, segment_batch_id, position);
COMMENT ON TABLE public.knowledge_segments IS '知识来源分段';
COMMENT ON COLUMN public.knowledge_segments.source_type IS '来源类型：document 文档、qa_entry 问答条目';
COMMENT ON COLUMN public.knowledge_segments.source_id IS '来源编号，对应文档或问答条目';
COMMENT ON COLUMN public.knowledge_segments.segment_batch_id IS '分段批次编号，与来源已发布批次一致';
COMMENT ON COLUMN public.knowledge_segments.position IS '分段在来源中的序号，从 1 开始';

-- +goose Down
DROP INDEX public.knowledge_segments_batch_position_unique;
DELETE FROM public.knowledge_segments WHERE source_type <> 'document';
ALTER TABLE public.knowledge_segments DROP COLUMN source_type;
ALTER TABLE public.knowledge_segments RENAME COLUMN source_id TO document_id;
CREATE UNIQUE INDEX knowledge_segments_batch_position_unique ON public.knowledge_segments (document_id, segment_batch_id, position);
COMMENT ON TABLE public.knowledge_segments IS '知识文档分段';
COMMENT ON COLUMN public.knowledge_segments.document_id IS '所属文档编号';
COMMENT ON COLUMN public.knowledge_segments.segment_batch_id IS '分段批次编号，与文档已发布批次一致';
COMMENT ON COLUMN public.knowledge_segments.position IS '分段在文档中的序号，从 1 开始';
