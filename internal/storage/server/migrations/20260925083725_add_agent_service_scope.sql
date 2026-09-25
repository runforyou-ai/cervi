-- +goose Up
-- AI 员工记录服务对象与转人工团队，接待客户开关只用于真人成员。
ALTER TABLE agents
    ADD COLUMN service_audiences text[] NOT NULL DEFAULT '{}'::text[],
    ADD COLUMN handoff_team_id uuid;

COMMENT ON COLUMN agents.service_audiences IS 'AI 员工的服务对象：customer 客户、employee 本企业员工；助理为空';
COMMENT ON COLUMN agents.handoff_team_id IS 'Cervi 内单聊与群话题转人工的团队编号，为空时进入公共队列';
COMMENT ON COLUMN organization_identities.handles_customers IS '真人成员是否接待客户';

-- +goose Down
COMMENT ON COLUMN organization_identities.handles_customers IS '是否接待客户';

ALTER TABLE agents
    DROP COLUMN handoff_team_id,
    DROP COLUMN service_audiences;
