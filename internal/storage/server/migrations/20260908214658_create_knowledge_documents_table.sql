-- +goose Up
CREATE TABLE knowledge_documents (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    knowledge_base_id uuid NOT NULL,
    group_id uuid NOT NULL,
    file_id uuid NOT NULL,
    status varchar(32) NOT NULL DEFAULT 'initial',
    created_by_user_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX knowledge_documents_file_unique ON knowledge_documents (file_id);
COMMENT ON TABLE knowledge_documents IS '知识库文件文档';
COMMENT ON COLUMN knowledge_documents.id IS '文档编号';
COMMENT ON COLUMN knowledge_documents.knowledge_base_id IS '所属知识库编号';
COMMENT ON COLUMN knowledge_documents.group_id IS '所属分组编号';
COMMENT ON COLUMN knowledge_documents.file_id IS '原件编号及上传重试幂等键';
COMMENT ON COLUMN knowledge_documents.status IS '文档处理状态';
COMMENT ON COLUMN knowledge_documents.created_by_user_id IS '创建用户编号';
COMMENT ON COLUMN knowledge_documents.created_at IS '创建时间';
COMMENT ON COLUMN knowledge_documents.updated_at IS '更新时间';

-- +goose Down
DROP TABLE knowledge_documents;
