-- +goose Up
-- 创建企业聊天主体表。
CREATE TABLE chat_subjects (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    organization_id  uuid NOT NULL,
    kind             text NOT NULL,
    source_id        uuid NOT NULL
);

COMMENT ON TABLE chat_subjects IS '企业聊天主体';
COMMENT ON COLUMN chat_subjects.id IS '聊天主体编号';
COMMENT ON COLUMN chat_subjects.created_at IS '创建时间';
COMMENT ON COLUMN chat_subjects.updated_at IS '更新时间';
COMMENT ON COLUMN chat_subjects.organization_id IS '所属企业编号';
COMMENT ON COLUMN chat_subjects.kind IS '聊天主体类型：organization_identity、contact';
COMMENT ON COLUMN chat_subjects.source_id IS '主体来源记录编号';

CREATE UNIQUE INDEX chat_subjects_org_kind_source_unique
    ON chat_subjects (organization_id, kind, source_id);

COMMENT ON INDEX chat_subjects_org_kind_source_unique
    IS '企业内聊天主体来源唯一索引';

-- +goose Down
DROP TABLE chat_subjects;
