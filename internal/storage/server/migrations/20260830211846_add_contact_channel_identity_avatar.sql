-- +goose Up
-- 增加渠道身份头像的外部引用和单调刷新位置。
ALTER TABLE contact_channel_identities
    ADD COLUMN avatar_external_id text,
    ADD COLUMN avatar_external_version text,
    ADD COLUMN avatar_source_order bigint NOT NULL DEFAULT 0;

COMMENT ON COLUMN contact_channel_identities.avatar_external_id IS '渠道头像的不透明外部引用';
COMMENT ON COLUMN contact_channel_identities.avatar_external_version IS '渠道头像的稳定版本摘要';
COMMENT ON COLUMN contact_channel_identities.avatar_source_order IS '最后头像刷新对应的来源内顺序';

-- +goose Down
ALTER TABLE contact_channel_identities
    DROP COLUMN avatar_source_order,
    DROP COLUMN avatar_external_version,
    DROP COLUMN avatar_external_id;
