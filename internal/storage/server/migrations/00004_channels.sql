-- +goose Up
CREATE TABLE channels (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid NOT NULL,
    created_by_user_id  uuid NOT NULL,
    type                text NOT NULL CHECK (btrim(type) <> ''),
    name                text NOT NULL CHECK (btrim(name) <> ''),
    description         text,
    default_locale      text NOT NULL DEFAULT 'zh-CN'
                            CHECK (default_locale IN ('zh-CN', 'en-US')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);

CREATE INDEX channels_organization_type_deleted_at_index
    ON channels (organization_id, type, deleted_at);

-- +goose Down
DROP TABLE channels;
