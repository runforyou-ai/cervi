-- +goose Up
-- 创建知识库分组表。
CREATE TABLE knowledge_groups (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    knowledge_base_id   uuid NOT NULL,
    parent_id           uuid,
    name                text NOT NULL,
    is_default          boolean NOT NULL DEFAULT false,
    sort_order          integer NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX knowledge_groups_sibling_name_unique
    ON knowledge_groups (
        knowledge_base_id,
        COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid),
        lower(name)
    )
    WHERE is_default = false;

CREATE UNIQUE INDEX knowledge_groups_default_unique
    ON knowledge_groups (knowledge_base_id)
    WHERE is_default = true;

COMMENT ON TABLE knowledge_groups IS '知识库分组';
COMMENT ON COLUMN knowledge_groups.id IS '分组编号';
COMMENT ON COLUMN knowledge_groups.created_at IS '创建时间';
COMMENT ON COLUMN knowledge_groups.updated_at IS '更新时间';
COMMENT ON COLUMN knowledge_groups.knowledge_base_id IS '所属知识库编号';
COMMENT ON COLUMN knowledge_groups.parent_id IS '上级分组编号，最多两级';
COMMENT ON COLUMN knowledge_groups.name IS '分组名称，默认分组为空';
COMMENT ON COLUMN knowledge_groups.is_default IS '是否为知识库默认分组';
COMMENT ON COLUMN knowledge_groups.sort_order IS '同级排序值';

-- +goose Down
DROP TABLE knowledge_groups;
