-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;
ALTER TABLE public.knowledge_segments
    ADD COLUMN embedding vector,
    ADD COLUMN embedding_dimension integer;
COMMENT ON COLUMN public.knowledge_segments.embedding IS '分段向量';
COMMENT ON COLUMN public.knowledge_segments.embedding_dimension IS '分段向量维度';
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

-- +goose Down
ALTER TABLE public.knowledge_segments
    DROP COLUMN embedding_dimension,
    DROP COLUMN embedding;
