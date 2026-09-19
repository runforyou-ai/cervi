-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE public.knowledge_segments (
    id varchar(128) PRIMARY KEY,
    organization_id uuid NOT NULL,
    knowledge_base_id uuid NOT NULL,
    source_type varchar(32) NOT NULL,
    source_id uuid NOT NULL,
    segment_batch_id uuid NOT NULL,
    position integer NOT NULL,
    character_count integer NOT NULL,
    context text NOT NULL,
    content text NOT NULL,
    search_vector tsvector,
    embedding vector,
    embedding_dimension integer
);
ALTER TABLE public.knowledge_segments ALTER COLUMN search_vector SET STATISTICS 3000;
CREATE UNIQUE INDEX knowledge_segments_batch_position_unique ON public.knowledge_segments (source_id, segment_batch_id, position);
CREATE INDEX knowledge_segments_knowledge_base_idx ON public.knowledge_segments (knowledge_base_id);
CREATE INDEX knowledge_segments_search_vector_idx ON public.knowledge_segments USING gin (search_vector);
CREATE INDEX knowledge_segments_embedding_384 ON public.knowledge_segments
    USING hnsw ((embedding::halfvec(384)) halfvec_cosine_ops) WHERE embedding_dimension = 384;
CREATE INDEX knowledge_segments_embedding_512 ON public.knowledge_segments
    USING hnsw ((embedding::halfvec(512)) halfvec_cosine_ops) WHERE embedding_dimension = 512;
CREATE INDEX knowledge_segments_embedding_768 ON public.knowledge_segments
    USING hnsw ((embedding::halfvec(768)) halfvec_cosine_ops) WHERE embedding_dimension = 768;
CREATE INDEX knowledge_segments_embedding_1024 ON public.knowledge_segments
    USING hnsw ((embedding::halfvec(1024)) halfvec_cosine_ops) WHERE embedding_dimension = 1024;
CREATE INDEX knowledge_segments_embedding_1536 ON public.knowledge_segments
    USING hnsw ((embedding::halfvec(1536)) halfvec_cosine_ops) WHERE embedding_dimension = 1536;
CREATE INDEX knowledge_segments_embedding_2048 ON public.knowledge_segments
    USING hnsw ((embedding::halfvec(2048)) halfvec_cosine_ops) WHERE embedding_dimension = 2048;
CREATE INDEX knowledge_segments_embedding_3072 ON public.knowledge_segments
    USING hnsw ((embedding::halfvec(3072)) halfvec_cosine_ops) WHERE embedding_dimension = 3072;
COMMENT ON TABLE public.knowledge_segments IS '知识来源分段';
COMMENT ON COLUMN public.knowledge_segments.id IS '分段唯一编号';
COMMENT ON COLUMN public.knowledge_segments.organization_id IS '所属企业编号';
COMMENT ON COLUMN public.knowledge_segments.knowledge_base_id IS '所属知识库编号';
COMMENT ON COLUMN public.knowledge_segments.source_type IS '来源类型：document 文档、qa_entry 问答条目';
COMMENT ON COLUMN public.knowledge_segments.source_id IS '来源编号，对应文档或问答条目';
COMMENT ON COLUMN public.knowledge_segments.segment_batch_id IS '分段批次编号，与来源已发布批次一致';
COMMENT ON COLUMN public.knowledge_segments.position IS '分段在来源中的序号，从 1 开始';
COMMENT ON COLUMN public.knowledge_segments.character_count IS '分段字符数';
COMMENT ON COLUMN public.knowledge_segments.context IS '分段起点所属的标题路径和表头，与正文共同用于向量、词法词元和重排';
COMMENT ON COLUMN public.knowledge_segments.content IS '分段完整正文';
COMMENT ON COLUMN public.knowledge_segments.search_vector IS '词法检索词元，由服务端按单字、编号和分词写入';
COMMENT ON COLUMN public.knowledge_segments.embedding IS '分段向量';
COMMENT ON COLUMN public.knowledge_segments.embedding_dimension IS '分段向量维度';

-- +goose Down
DROP TABLE public.knowledge_segments;
