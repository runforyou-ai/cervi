-- +goose Up
-- 为 agents 增加 AI 员工负责人。
ALTER TABLE agents
    ADD COLUMN responsible_user_id uuid;

COMMENT ON COLUMN agents.responsible_user_id IS 'AI 员工负责人编号，为空表示未指定，助理为空';

-- +goose Down
ALTER TABLE agents
    DROP COLUMN responsible_user_id;
