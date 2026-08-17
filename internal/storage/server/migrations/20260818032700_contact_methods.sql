-- +goose Up
-- 创建联系人联系方式表，类型和内容由 Action 校验。
CREATE TABLE contact_methods (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL,
    contact_id        uuid NOT NULL,
    type              text NOT NULL,
    value             text NOT NULL,
    normalized_value  text NOT NULL,
    label             text,
    is_primary        boolean NOT NULL DEFAULT false,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX contact_methods_organization_contact_index
    ON contact_methods (organization_id, contact_id);

CREATE INDEX contact_methods_organization_type_value_index
    ON contact_methods (organization_id, type, normalized_value);

CREATE UNIQUE INDEX contact_methods_contact_type_value_unique
    ON contact_methods (contact_id, type, normalized_value);

CREATE UNIQUE INDEX contact_methods_contact_primary_type_unique
    ON contact_methods (contact_id, type)
    WHERE is_primary;

-- +goose Down
DROP TABLE contact_methods;
