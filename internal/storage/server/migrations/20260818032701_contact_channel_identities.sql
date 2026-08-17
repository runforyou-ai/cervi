-- +goose Up
-- 创建联系人渠道身份表，关联关系由 Action 维护。
CREATE TABLE contact_channel_identities (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL,
    contact_id       uuid NOT NULL,
    channel_id       uuid NOT NULL,
    external_id      text NOT NULL,
    display_name     text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    last_seen_at     timestamptz
);

CREATE UNIQUE INDEX contact_channel_identities_channel_external_unique
    ON contact_channel_identities (channel_id, external_id);

CREATE INDEX contact_channel_identities_organization_contact_index
    ON contact_channel_identities (organization_id, contact_id);

CREATE INDEX contact_channel_identities_organization_channel_contact_index
    ON contact_channel_identities (organization_id, channel_id, contact_id);

-- +goose Down
DROP TABLE contact_channel_identities;
