-- +goose Up
-- 服务周期记录接待的 AI 员工，待补知识按来源周期归属。
ALTER TABLE service_sessions
    ADD COLUMN agent_identity_id uuid;

COMMENT ON COLUMN service_sessions.agent_identity_id IS '接待该周期的 AI 员工企业身份编号：开启时或之后首次负责该周期的 AI 员工，从未由 AI 员工负责时为空';

ALTER TABLE knowledge_gaps
    DROP COLUMN agent_identity_id;

-- +goose Down
ALTER TABLE knowledge_gaps
    ADD COLUMN agent_identity_id uuid;

COMMENT ON COLUMN knowledge_gaps.agent_identity_id IS '接待该周期的 AI 员工企业身份编号，用于选择默认知识库';

ALTER TABLE service_sessions
    DROP COLUMN agent_identity_id;
