-- +goose Up
-- 增加渠道身份的企业头像文件，并记录服务端导入文件的来源键。
ALTER TABLE contact_channel_identities
    ADD COLUMN avatar_file_id uuid;

COMMENT ON COLUMN contact_channel_identities.avatar_file_id IS '渠道头像文件编号';

ALTER TABLE files
    ADD COLUMN external_id text;

COMMENT ON COLUMN files.external_id IS '外部来源的文件唯一标识';

CREATE INDEX files_organization_purpose_external_id_index
    ON files (organization_id, purpose, external_id)
    WHERE external_id IS NOT NULL;

-- +goose Down
DROP INDEX files_organization_purpose_external_id_index;

ALTER TABLE files
    DROP COLUMN external_id;

ALTER TABLE contact_channel_identities
    DROP COLUMN avatar_file_id;
