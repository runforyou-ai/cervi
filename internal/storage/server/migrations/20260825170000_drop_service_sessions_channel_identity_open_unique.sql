-- +goose Up
-- 允许网站渠道身份同时处理多条客户会话。
DROP INDEX service_sessions_org_channel_identity_open_unique;

-- +goose Down
CREATE UNIQUE INDEX service_sessions_org_channel_identity_open_unique
    ON service_sessions (organization_id, contact_channel_identity_id)
    WHERE status IN ('waiting', 'active', 'pending');

COMMENT ON INDEX service_sessions_org_channel_identity_open_unique
    IS '企业渠道身份未结束处理周期唯一索引';
