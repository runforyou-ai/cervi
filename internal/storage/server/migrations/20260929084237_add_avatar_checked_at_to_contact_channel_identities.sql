-- +goose Up
-- 为联系人渠道身份增加头像同步时间。
ALTER TABLE contact_channel_identities ADD COLUMN avatar_checked_at timestamptz;

COMMENT ON COLUMN contact_channel_identities.avatar_checked_at IS '最近一次发起渠道头像同步的时间，从未同步时为空';

-- +goose Down
ALTER TABLE contact_channel_identities DROP COLUMN avatar_checked_at;
