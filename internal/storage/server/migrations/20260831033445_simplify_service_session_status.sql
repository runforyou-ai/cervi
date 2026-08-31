-- +goose Up
-- 客服处理周期只保留开启和关闭状态，队列归属由负责人字段表达。
DROP INDEX service_sessions_organization_conversation_open_unique;

UPDATE service_sessions
SET status = 'open',
    status_changed_at = now(),
    updated_at = now()
WHERE status IN ('waiting', 'active', 'pending');

ALTER TABLE service_sessions
    ALTER COLUMN status SET DEFAULT 'open';

COMMENT ON COLUMN service_sessions.status IS '客服处理状态：open、closed';

CREATE UNIQUE INDEX service_sessions_organization_conversation_open_unique
    ON service_sessions (organization_id, conversation_id)
    WHERE status = 'open';

COMMENT ON INDEX service_sessions_organization_conversation_open_unique
    IS '企业客户会话未结束处理周期唯一索引';

-- +goose Down
DROP INDEX service_sessions_organization_conversation_open_unique;

UPDATE service_sessions
SET status = CASE
        WHEN assignee_identity_id IS NULL THEN 'waiting'
        ELSE 'active'
    END,
    status_changed_at = now(),
    updated_at = now()
WHERE status = 'open';

ALTER TABLE service_sessions
    ALTER COLUMN status SET DEFAULT 'waiting';

COMMENT ON COLUMN service_sessions.status IS '客服处理状态：waiting、active、pending、closed';

CREATE UNIQUE INDEX service_sessions_organization_conversation_open_unique
    ON service_sessions (organization_id, conversation_id)
    WHERE status IN ('waiting', 'active', 'pending');

COMMENT ON INDEX service_sessions_organization_conversation_open_unique
    IS '企业客户会话未结束处理周期唯一索引';
