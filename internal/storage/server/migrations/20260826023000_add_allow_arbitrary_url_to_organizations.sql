-- +goose Up
ALTER TABLE organizations
    ADD COLUMN allow_arbitrary_url boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN organizations.allow_arbitrary_url IS '是否允许成员打开任意网址';

-- +goose Down
ALTER TABLE organizations
    DROP COLUMN allow_arbitrary_url;
