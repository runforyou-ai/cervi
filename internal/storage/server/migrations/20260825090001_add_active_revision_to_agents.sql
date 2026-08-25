-- +goose Up
-- 增加 AI 员工当前配置版本。
ALTER TABLE agents
    ADD COLUMN active_revision_id uuid NOT NULL;

COMMENT ON COLUMN agents.active_revision_id IS '当前配置版本编号';

CREATE UNIQUE INDEX agents_organization_active_revision_unique
    ON agents (organization_id, active_revision_id);

-- +goose Down
DROP INDEX agents_organization_active_revision_unique;

ALTER TABLE agents
    DROP COLUMN active_revision_id;
