-- +goose Up
-- 创建 AI 员工配置版本表。
CREATE TABLE agent_revisions (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id     uuid NOT NULL,
    agent_id            uuid NOT NULL,
    provider_id         uuid NOT NULL,
    model_identifier    text NOT NULL,
    system_instruction  text NOT NULL,
    created_by_user_id  uuid NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE agent_revisions IS 'AI 员工不可变配置版本';
COMMENT ON COLUMN agent_revisions.id IS '配置版本编号';
COMMENT ON COLUMN agent_revisions.organization_id IS '所属企业编号';
COMMENT ON COLUMN agent_revisions.agent_id IS 'AI 员工编号';
COMMENT ON COLUMN agent_revisions.provider_id IS '模型服务供应商编号';
COMMENT ON COLUMN agent_revisions.model_identifier IS '对话模型标识';
COMMENT ON COLUMN agent_revisions.system_instruction IS '工作指令';
COMMENT ON COLUMN agent_revisions.created_by_user_id IS '创建用户编号';
COMMENT ON COLUMN agent_revisions.created_at IS '创建时间';

CREATE INDEX agent_revisions_organization_agent_index
    ON agent_revisions (organization_id, agent_id, created_at DESC);

CREATE INDEX agent_revisions_organization_provider_model_index
    ON agent_revisions (organization_id, provider_id, model_identifier);

-- +goose Down
DROP TABLE agent_revisions;
