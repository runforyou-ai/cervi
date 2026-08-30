-- +goose Up
-- 把渠道头像从远端引用切换为企业文件引用。
ALTER TABLE contact_channel_identities
    DROP COLUMN avatar_external_id,
    ADD COLUMN avatar_file_id uuid;

COMMENT ON COLUMN contact_channel_identities.avatar_file_id IS '渠道头像文件编号';

-- +goose Down
ALTER TABLE contact_channel_identities
    DROP COLUMN avatar_file_id,
    ADD COLUMN avatar_external_id text;

COMMENT ON COLUMN contact_channel_identities.avatar_external_id IS '渠道头像的不透明外部引用';
