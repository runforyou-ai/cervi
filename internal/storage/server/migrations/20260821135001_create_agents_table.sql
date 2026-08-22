-- +goose Up
-- 创建 AI 员工表。
CREATE TABLE agents (
    id                  uuid PRIMARY KEY,
    organization_id     uuid NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE agents IS 'AI 员工';
COMMENT ON COLUMN agents.id IS '成员编号';
COMMENT ON COLUMN agents.organization_id IS '所属企业编号';
COMMENT ON COLUMN agents.created_at IS '创建时间';
COMMENT ON COLUMN agents.updated_at IS '更新时间';

CREATE INDEX agents_organization_index
    ON agents (organization_id, id);

-- +goose Down
DROP TABLE agents;
