-- +goose Up
-- 创建 AI 员工表。
CREATE TABLE agents (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    identity_id         uuid NOT NULL,
    organization_id     uuid NOT NULL,
    status              text NOT NULL DEFAULT 'active',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE agents IS 'AI 员工';
COMMENT ON COLUMN agents.id IS 'AI 员工编号';
COMMENT ON COLUMN agents.identity_id IS '企业身份编号';
COMMENT ON COLUMN agents.organization_id IS '所属企业编号';
COMMENT ON COLUMN agents.status IS '启用状态';
COMMENT ON COLUMN agents.created_at IS '创建时间';
COMMENT ON COLUMN agents.updated_at IS '更新时间';

CREATE UNIQUE INDEX agents_organization_identity_unique
    ON agents (organization_id, identity_id);

-- +goose Down
DROP TABLE agents;
