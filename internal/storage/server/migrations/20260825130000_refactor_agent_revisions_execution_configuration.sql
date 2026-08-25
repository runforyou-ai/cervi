-- +goose Up
-- 将 AI 员工配置版本调整为可扩展的执行配置快照。
DROP INDEX agent_revisions_organization_provider_model_index;

ALTER TABLE agent_revisions
    DROP COLUMN provider_id,
    DROP COLUMN model_identifier,
    DROP COLUMN system_instruction,
    ADD COLUMN execution_mode text NOT NULL,
    ADD COLUMN schema_version integer NOT NULL,
    ADD COLUMN configuration jsonb NOT NULL;

COMMENT ON COLUMN agent_revisions.execution_mode IS '执行方式';
COMMENT ON COLUMN agent_revisions.schema_version IS '配置结构版本';
COMMENT ON COLUMN agent_revisions.configuration IS '非敏感执行配置快照';

-- +goose Down
ALTER TABLE agent_revisions
    DROP COLUMN execution_mode,
    DROP COLUMN schema_version,
    DROP COLUMN configuration,
    ADD COLUMN provider_id uuid NOT NULL,
    ADD COLUMN model_identifier text NOT NULL,
    ADD COLUMN system_instruction text NOT NULL;

COMMENT ON COLUMN agent_revisions.provider_id IS '模型服务供应商编号';
COMMENT ON COLUMN agent_revisions.model_identifier IS '对话模型标识';
COMMENT ON COLUMN agent_revisions.system_instruction IS '工作指令';

CREATE INDEX agent_revisions_organization_provider_model_index
    ON agent_revisions (organization_id, provider_id, model_identifier);
