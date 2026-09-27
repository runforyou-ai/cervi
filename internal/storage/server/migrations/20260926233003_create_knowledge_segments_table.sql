-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

-- 创建知识来源分段表。
CREATE TABLE knowledge_segments (
    id                   varchar(128) PRIMARY KEY,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    organization_id      uuid NOT NULL,
    knowledge_base_id    uuid NOT NULL,
    source_type          varchar(32) NOT NULL,
    source_id            uuid NOT NULL,
    segment_batch_id     uuid NOT NULL,
    "position"           integer NOT NULL,
    character_count      integer NOT NULL,
    context              text NOT NULL,
    content              text NOT NULL,
    search_vector        tsvector,
    embedding            vector,
    embedding_dimension  integer
);

CREATE UNIQUE INDEX knowledge_segments_batch_position_unique
    ON knowledge_segments (source_id, segment_batch_id, "position");

CREATE INDEX knowledge_segments_embedding_384
    ON knowledge_segments USING hnsw ((embedding::halfvec(384)) halfvec_cosine_ops) WHERE (embedding_dimension = 384);

CREATE INDEX knowledge_segments_embedding_512
    ON knowledge_segments USING hnsw ((embedding::halfvec(512)) halfvec_cosine_ops) WHERE (embedding_dimension = 512);

CREATE INDEX knowledge_segments_embedding_768
    ON knowledge_segments USING hnsw ((embedding::halfvec(768)) halfvec_cosine_ops) WHERE (embedding_dimension = 768);

CREATE INDEX knowledge_segments_embedding_1024
    ON knowledge_segments USING hnsw ((embedding::halfvec(1024)) halfvec_cosine_ops) WHERE (embedding_dimension = 1024);

CREATE INDEX knowledge_segments_embedding_1536
    ON knowledge_segments USING hnsw ((embedding::halfvec(1536)) halfvec_cosine_ops) WHERE (embedding_dimension = 1536);

CREATE INDEX knowledge_segments_embedding_2048
    ON knowledge_segments USING hnsw ((embedding::halfvec(2048)) halfvec_cosine_ops) WHERE (embedding_dimension = 2048);

CREATE INDEX knowledge_segments_embedding_3072
    ON knowledge_segments USING hnsw ((embedding::halfvec(3072)) halfvec_cosine_ops) WHERE (embedding_dimension = 3072);

CREATE INDEX knowledge_segments_knowledge_base_idx
    ON knowledge_segments (knowledge_base_id);

CREATE INDEX knowledge_segments_search_vector_idx
    ON knowledge_segments USING gin (search_vector);

ALTER TABLE knowledge_segments ALTER COLUMN search_vector SET STATISTICS 3000;

COMMENT ON TABLE knowledge_segments IS '知识来源分段';
COMMENT ON COLUMN knowledge_segments.id IS '分段唯一编号';
COMMENT ON COLUMN knowledge_segments.created_at IS '创建时间';
COMMENT ON COLUMN knowledge_segments.updated_at IS '更新时间';
COMMENT ON COLUMN knowledge_segments.organization_id IS '所属工作区编号';
COMMENT ON COLUMN knowledge_segments.knowledge_base_id IS '所属知识库编号';
COMMENT ON COLUMN knowledge_segments.source_type IS '来源类型：document 文档、qa_entry 问答条目';
COMMENT ON COLUMN knowledge_segments.source_id IS '来源编号，对应文档或问答条目';
COMMENT ON COLUMN knowledge_segments.segment_batch_id IS '分段批次编号，与来源已发布批次一致';
COMMENT ON COLUMN knowledge_segments.character_count IS '分段字符数';
COMMENT ON COLUMN knowledge_segments.context IS '分段起点所属的标题路径和表头，与正文共同用于向量、词法词元和重排';
COMMENT ON COLUMN knowledge_segments.content IS '分段完整正文';
COMMENT ON COLUMN knowledge_segments.search_vector IS '词法检索词元，由服务端按单字、编号和分词写入';
COMMENT ON COLUMN knowledge_segments.embedding IS '分段向量';
COMMENT ON COLUMN knowledge_segments.embedding_dimension IS '分段向量维度';

-- +goose Down
DROP TABLE knowledge_segments;
