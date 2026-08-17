-- +goose Up
CREATE TABLE contacts (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid NOT NULL,
    created_by_user_id  uuid NOT NULL,
    source_channel_id   uuid,
    display_name        text,
    stage               text NOT NULL DEFAULT 'visitor'
                            CHECK (stage IN ('visitor', 'lead', 'customer')),
    notes               text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);

CREATE INDEX contacts_organization_deleted_stage_updated_index
    ON contacts (organization_id, deleted_at, stage, updated_at DESC, id DESC);

CREATE INDEX contacts_organization_display_name_index
    ON contacts (organization_id, lower(display_name));

CREATE INDEX contacts_organization_source_channel_index
    ON contacts (organization_id, source_channel_id, deleted_at);

-- +goose Down
DROP TABLE contacts;
