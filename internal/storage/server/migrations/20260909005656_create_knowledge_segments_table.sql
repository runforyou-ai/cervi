-- +goose Up
CREATE TABLE public.knowledge_segments (
    id varchar(128) PRIMARY KEY,
    content text NOT NULL,
    meta jsonb NOT NULL
);
CREATE UNIQUE INDEX knowledge_segments_batch_position_unique ON public.knowledge_segments
    ((meta->>'document_id'), (meta->>'batch_id'), ((meta->>'position')::integer));
COMMENT ON TABLE public.knowledge_segments IS '知识文档分段';
COMMENT ON COLUMN public.knowledge_segments.id IS '分段唯一编号';
COMMENT ON COLUMN public.knowledge_segments.content IS '分段完整正文';
COMMENT ON COLUMN public.knowledge_segments.meta IS '企业、知识库、文档、批次、位置及来源信息';

-- +goose Down
DROP TABLE public.knowledge_segments;
