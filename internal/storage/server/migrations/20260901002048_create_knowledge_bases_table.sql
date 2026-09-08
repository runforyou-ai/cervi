-- +goose Up
-- 创建企业知识库表。
CREATE TABLE knowledge_bases (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    organization_id     uuid NOT NULL,
    created_by_user_id  uuid NOT NULL,
    name                text NOT NULL,
    category            text NOT NULL,
    description         text NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX knowledge_bases_organization_name_unique
    ON knowledge_bases (organization_id, lower(name));

COMMENT ON TABLE knowledge_bases IS '企业知识库';
COMMENT ON COLUMN knowledge_bases.id IS '知识库编号';
COMMENT ON COLUMN knowledge_bases.created_at IS '创建时间';
COMMENT ON COLUMN knowledge_bases.updated_at IS '更新时间';
COMMENT ON COLUMN knowledge_bases.organization_id IS '所属企业编号';
COMMENT ON COLUMN knowledge_bases.created_by_user_id IS '创建用户编号';
COMMENT ON COLUMN knowledge_bases.name IS '知识库名称';
COMMENT ON COLUMN knowledge_bases.category IS '内容类型：standard 文档库、qa 问答库';
COMMENT ON COLUMN knowledge_bases.description IS '知识库描述';

-- +goose Down
DROP TABLE knowledge_bases;
