-- +goose Up
CREATE TABLE settings (
    organization_id  uuid NOT NULL,
    key              text NOT NULL CHECK (btrim(key) <> ''),
    value            jsonb NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (organization_id, key)
);

-- +goose Down
DROP TABLE settings;
