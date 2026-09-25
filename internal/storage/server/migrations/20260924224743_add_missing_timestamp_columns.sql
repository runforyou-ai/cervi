-- +goose Up
-- 为缺少创建时间或更新时间的表补齐时间列，已有行取迁移执行时间。
ALTER TABLE tokens ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE team_members ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE role_permissions ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE ai_provider_models ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE agent_revisions ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE agent_inputs ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE operator_provisionings ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE acme_cache_entries ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE customer_channel_send_gates ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE conversation_mention_reviews
    ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE agent_run_blocks
    ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE message_attachments
    ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE channel_messages
    ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE knowledge_segments
    ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

COMMENT ON COLUMN acme_cache_entries.created_at IS '创建时间';
COMMENT ON COLUMN customer_channel_send_gates.created_at IS '创建时间';
COMMENT ON COLUMN conversation_mention_reviews.created_at IS '创建时间';
COMMENT ON COLUMN agent_run_blocks.created_at IS '创建时间';
COMMENT ON COLUMN message_attachments.created_at IS '创建时间';
COMMENT ON COLUMN channel_messages.created_at IS '创建时间';
COMMENT ON COLUMN knowledge_segments.created_at IS '创建时间';
COMMENT ON COLUMN tokens.updated_at IS '更新时间';
COMMENT ON COLUMN team_members.updated_at IS '更新时间';
COMMENT ON COLUMN role_permissions.updated_at IS '更新时间';
COMMENT ON COLUMN ai_provider_models.updated_at IS '更新时间';
COMMENT ON COLUMN agent_revisions.updated_at IS '更新时间';
COMMENT ON COLUMN agent_inputs.updated_at IS '更新时间';
COMMENT ON COLUMN operator_provisionings.updated_at IS '更新时间';
COMMENT ON COLUMN conversation_mention_reviews.updated_at IS '更新时间';
COMMENT ON COLUMN agent_run_blocks.updated_at IS '更新时间';
COMMENT ON COLUMN message_attachments.updated_at IS '更新时间';
COMMENT ON COLUMN channel_messages.updated_at IS '更新时间';
COMMENT ON COLUMN knowledge_segments.updated_at IS '更新时间';

-- +goose Down
ALTER TABLE knowledge_segments
    DROP COLUMN updated_at,
    DROP COLUMN created_at;
ALTER TABLE channel_messages
    DROP COLUMN updated_at,
    DROP COLUMN created_at;
ALTER TABLE message_attachments
    DROP COLUMN updated_at,
    DROP COLUMN created_at;
ALTER TABLE agent_run_blocks
    DROP COLUMN updated_at,
    DROP COLUMN created_at;
ALTER TABLE conversation_mention_reviews
    DROP COLUMN updated_at,
    DROP COLUMN created_at;
ALTER TABLE customer_channel_send_gates DROP COLUMN created_at;
ALTER TABLE acme_cache_entries DROP COLUMN created_at;
ALTER TABLE operator_provisionings DROP COLUMN updated_at;
ALTER TABLE agent_inputs DROP COLUMN updated_at;
ALTER TABLE agent_revisions DROP COLUMN updated_at;
ALTER TABLE ai_provider_models DROP COLUMN updated_at;
ALTER TABLE role_permissions DROP COLUMN updated_at;
ALTER TABLE team_members DROP COLUMN updated_at;
ALTER TABLE tokens DROP COLUMN updated_at;
