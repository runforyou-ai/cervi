-- +goose Up
ALTER TABLE agent_evaluation_cases
    ADD COLUMN source text NOT NULL DEFAULT 'manual',
    ADD COLUMN service_session_id uuid,
    ADD COLUMN question_message_id uuid,
    ADD COLUMN occurred_at timestamptz,
    ADD COLUMN context jsonb;

CREATE UNIQUE INDEX agent_evaluation_cases_question_message_unique
    ON agent_evaluation_cases (organization_id, question_message_id) WHERE (question_message_id IS NOT NULL);

COMMENT ON COLUMN agent_evaluation_cases.source IS '来源：manual 手动、knowledge_gap 待补知识、service_session 问题会话';
COMMENT ON COLUMN agent_evaluation_cases.service_session_id IS '来源服务周期编号，手动用例为空';
COMMENT ON COLUMN agent_evaluation_cases.question_message_id IS '来源周期中作为提问的客户消息编号，手动用例为空';
COMMENT ON COLUMN agent_evaluation_cases.occurred_at IS '所选提问消息的时间，手动用例为空';
COMMENT ON COLUMN agent_evaluation_cases.context IS '加入时的快照：前文、客户上下文正文、客户已验证身份与来源是否为渠道会话，手动用例为空';
COMMENT ON INDEX agent_evaluation_cases_question_message_unique IS '同一提问消息只加入一条评测用例';

-- +goose Down
DROP INDEX agent_evaluation_cases_question_message_unique;

ALTER TABLE agent_evaluation_cases
    DROP COLUMN context,
    DROP COLUMN occurred_at,
    DROP COLUMN question_message_id,
    DROP COLUMN service_session_id,
    DROP COLUMN source;
