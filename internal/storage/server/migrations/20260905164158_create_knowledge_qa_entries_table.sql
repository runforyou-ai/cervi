-- +goose Up
CREATE TABLE knowledge_qa_entries (
    id                    uuid PRIMARY KEY DEFAULT uuidv7(),
    knowledge_base_id     uuid NOT NULL,
    group_id              uuid NOT NULL,
    created_by_user_id    uuid NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE knowledge_qa_entries IS '知识问答条目';
COMMENT ON COLUMN knowledge_qa_entries.id IS '问答编号';
COMMENT ON COLUMN knowledge_qa_entries.knowledge_base_id IS '所属知识库编号';
COMMENT ON COLUMN knowledge_qa_entries.group_id IS '所属分组编号';
COMMENT ON COLUMN knowledge_qa_entries.created_by_user_id IS '创建用户编号';
COMMENT ON COLUMN knowledge_qa_entries.created_at IS '创建时间';
COMMENT ON COLUMN knowledge_qa_entries.updated_at IS '更新时间';

-- +goose Down
DROP TABLE knowledge_qa_entries;
