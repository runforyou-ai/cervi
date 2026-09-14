-- +goose Up
ALTER TABLE conversations
    ADD COLUMN version bigint NOT NULL DEFAULT 0;
ALTER TABLE conversation_user_states
    ADD COLUMN version bigint NOT NULL DEFAULT 0;
ALTER TABLE users
    ADD COLUMN profile_version bigint NOT NULL DEFAULT 0;
COMMENT ON COLUMN conversations.version IS '会话可见变化版本，在会话锁内单调推进';
COMMENT ON COLUMN conversation_user_states.version IS '用户会话个人状态版本，已读、提及确认、静音和手动未读实际变化时推进';
COMMENT ON COLUMN users.profile_version IS '用户身份资料与账户偏好版本，登录态可见字段实际变化时推进';

-- +goose Down
ALTER TABLE users
    DROP COLUMN profile_version;
ALTER TABLE conversation_user_states
    DROP COLUMN version;
ALTER TABLE conversations
    DROP COLUMN version;
