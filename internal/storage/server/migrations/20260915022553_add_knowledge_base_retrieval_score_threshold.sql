-- +goose Up
ALTER TABLE knowledge_bases
    ADD COLUMN retrieval_score_threshold double precision NOT NULL DEFAULT 0.7;
ALTER TABLE knowledge_bases
    ALTER COLUMN retrieval_score_threshold DROP DEFAULT;

COMMENT ON COLUMN knowledge_bases.retrieval_score_threshold IS '相关性阈值，重排得分低于该值的内容不返回';

-- +goose Down
ALTER TABLE knowledge_bases
    DROP COLUMN retrieval_score_threshold;
