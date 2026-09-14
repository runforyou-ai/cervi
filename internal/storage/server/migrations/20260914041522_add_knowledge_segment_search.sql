-- +goose Up
DROP INDEX public.knowledge_segments_batch_position_unique;
ALTER TABLE public.knowledge_segments
    ADD COLUMN organization_id uuid,
    ADD COLUMN knowledge_base_id uuid,
    ADD COLUMN document_id uuid,
    ADD COLUMN segment_batch_id uuid,
    ADD COLUMN position integer,
    ADD COLUMN character_count integer,
    ADD COLUMN search_vector tsvector;
UPDATE public.knowledge_segments SET
    organization_id = (meta->>'organization_id')::uuid,
    knowledge_base_id = (meta->>'knowledge_base_id')::uuid,
    document_id = (meta->>'document_id')::uuid,
    segment_batch_id = (meta->>'batch_id')::uuid,
    position = (meta->>'position')::integer,
    character_count = (meta->>'character_count')::integer;
ALTER TABLE public.knowledge_segments
    ALTER COLUMN organization_id SET NOT NULL,
    ALTER COLUMN knowledge_base_id SET NOT NULL,
    ALTER COLUMN document_id SET NOT NULL,
    ALTER COLUMN segment_batch_id SET NOT NULL,
    ALTER COLUMN position SET NOT NULL,
    ALTER COLUMN character_count SET NOT NULL,
    ALTER COLUMN search_vector SET STATISTICS 3000,
    DROP COLUMN meta;
CREATE UNIQUE INDEX knowledge_segments_batch_position_unique ON public.knowledge_segments (document_id, segment_batch_id, position);
CREATE INDEX knowledge_segments_knowledge_base_idx ON public.knowledge_segments (knowledge_base_id);
CREATE INDEX knowledge_segments_search_vector_idx ON public.knowledge_segments USING gin (search_vector);
COMMENT ON COLUMN public.knowledge_segments.organization_id IS '所属企业编号';
COMMENT ON COLUMN public.knowledge_segments.knowledge_base_id IS '所属知识库编号';
COMMENT ON COLUMN public.knowledge_segments.document_id IS '所属文档编号';
COMMENT ON COLUMN public.knowledge_segments.segment_batch_id IS '分段批次编号，与文档已发布批次一致';
COMMENT ON COLUMN public.knowledge_segments.position IS '分段在文档中的序号，从 1 开始';
COMMENT ON COLUMN public.knowledge_segments.character_count IS '分段字符数';
COMMENT ON COLUMN public.knowledge_segments.search_vector IS '词法检索词元，由服务端按单字、编号和分词写入';

-- +goose Down
DROP INDEX public.knowledge_segments_search_vector_idx;
DROP INDEX public.knowledge_segments_knowledge_base_idx;
DROP INDEX public.knowledge_segments_batch_position_unique;
ALTER TABLE public.knowledge_segments ADD COLUMN meta jsonb;
UPDATE public.knowledge_segments SET meta = jsonb_build_object(
    'organization_id', organization_id::text, 'knowledge_base_id', knowledge_base_id::text,
    'document_id', document_id::text, 'batch_id', segment_batch_id::text,
    'position', position, 'character_count', character_count);
ALTER TABLE public.knowledge_segments
    ALTER COLUMN meta SET NOT NULL,
    DROP COLUMN search_vector,
    DROP COLUMN character_count,
    DROP COLUMN position,
    DROP COLUMN segment_batch_id,
    DROP COLUMN document_id,
    DROP COLUMN knowledge_base_id,
    DROP COLUMN organization_id;
CREATE UNIQUE INDEX knowledge_segments_batch_position_unique ON public.knowledge_segments
    ((meta->>'document_id'), (meta->>'batch_id'), ((meta->>'position')::integer));
