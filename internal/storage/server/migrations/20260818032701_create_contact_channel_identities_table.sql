-- +goose Up
-- 创建联系人渠道身份表。
CREATE TABLE contact_channel_identities (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id  uuid NOT NULL,
    contact_id       uuid NOT NULL,
    channel_id       uuid NOT NULL,
    external_id      text NOT NULL,
    display_name     text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    last_seen_at     timestamptz
);

COMMENT ON TABLE contact_channel_identities IS '联系人在外部渠道中的身份';
COMMENT ON COLUMN contact_channel_identities.id IS '渠道身份编号';
COMMENT ON COLUMN contact_channel_identities.organization_id IS '所属企业编号';
COMMENT ON COLUMN contact_channel_identities.contact_id IS '联系人编号';
COMMENT ON COLUMN contact_channel_identities.channel_id IS '渠道编号';
COMMENT ON COLUMN contact_channel_identities.external_id IS '联系人在渠道中的外部编号';
COMMENT ON COLUMN contact_channel_identities.display_name IS '联系人在渠道中的显示名称';
COMMENT ON COLUMN contact_channel_identities.created_at IS '创建时间';
COMMENT ON COLUMN contact_channel_identities.updated_at IS '更新时间';
COMMENT ON COLUMN contact_channel_identities.last_seen_at IS '最后活跃时间';

CREATE UNIQUE INDEX contact_channel_identities_channel_external_unique
    ON contact_channel_identities (channel_id, external_id);

CREATE INDEX contact_channel_identities_organization_contact_index
    ON contact_channel_identities (organization_id, contact_id);

CREATE INDEX contact_channel_identities_organization_channel_contact_index
    ON contact_channel_identities (organization_id, channel_id, contact_id);

-- +goose Down
DROP TABLE contact_channel_identities;
