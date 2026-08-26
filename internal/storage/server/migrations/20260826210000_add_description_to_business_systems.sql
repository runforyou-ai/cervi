-- +goose Up
ALTER TABLE business_systems
    ADD COLUMN description text NOT NULL DEFAULT '';

COMMENT ON COLUMN business_systems.description IS '业务系统描述';

-- +goose Down
ALTER TABLE business_systems
    DROP COLUMN description;
