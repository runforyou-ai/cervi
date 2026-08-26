-- +goose Up
-- 统一索引命名中的企业前缀：org 缩写改为主流的 organization 全称。
-- customer_conversations_org_channel_identity_created_index、
-- conversation_participants_org_conversation_subject_unique、
-- service_sessions_org_channel_identity_last_message_index
-- 改全称后超出 PostgreSQL 63 字符标识符上限，保留缩写不重命名。
-- service_sessions_org_channel_identity_open_unique 已在
-- 20260825170000 迁移中删除，无需处理。
ALTER INDEX chat_subjects_org_kind_source_unique
    RENAME TO chat_subjects_organization_kind_source_unique;

ALTER INDEX conversations_org_type_status_last_message_index
    RENAME TO conversations_organization_type_status_last_message_index;

ALTER INDEX conversation_participants_org_subject_active_index
    RENAME TO conversation_participants_organization_subject_active_index;

ALTER INDEX service_sessions_org_conversation_sequence_unique
    RENAME TO service_sessions_organization_conversation_sequence_unique;

ALTER INDEX service_sessions_org_opening_message_unique
    RENAME TO service_sessions_organization_opening_message_unique;

ALTER INDEX service_sessions_org_conversation_open_unique
    RENAME TO service_sessions_organization_conversation_open_unique;

ALTER INDEX service_sessions_org_conversation_last_message_index
    RENAME TO service_sessions_organization_conversation_last_message_index;

ALTER INDEX messages_org_conversation_originated_index
    RENAME TO messages_organization_conversation_originated_index;

ALTER INDEX messages_org_service_session_originated_index
    RENAME TO messages_organization_service_session_originated_index;

-- +goose Down
-- 恢复原有的 org 缩写命名。
ALTER INDEX chat_subjects_organization_kind_source_unique
    RENAME TO chat_subjects_org_kind_source_unique;

ALTER INDEX conversations_organization_type_status_last_message_index
    RENAME TO conversations_org_type_status_last_message_index;

ALTER INDEX conversation_participants_organization_subject_active_index
    RENAME TO conversation_participants_org_subject_active_index;

ALTER INDEX service_sessions_organization_conversation_sequence_unique
    RENAME TO service_sessions_org_conversation_sequence_unique;

ALTER INDEX service_sessions_organization_opening_message_unique
    RENAME TO service_sessions_org_opening_message_unique;

ALTER INDEX service_sessions_organization_conversation_open_unique
    RENAME TO service_sessions_org_conversation_open_unique;

ALTER INDEX service_sessions_organization_conversation_last_message_index
    RENAME TO service_sessions_org_conversation_last_message_index;

ALTER INDEX messages_organization_conversation_originated_index
    RENAME TO messages_org_conversation_originated_index;

ALTER INDEX messages_organization_service_session_originated_index
    RENAME TO messages_org_service_session_originated_index;
