-- +goose Up
-- 创建工作区联系人标签表。
CREATE TABLE contact_tags (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    organization_id  uuid NOT NULL,
    name             text NOT NULL
);

CREATE UNIQUE INDEX contact_tags_organization_name_unique
    ON contact_tags (organization_id, lower(name));

COMMENT ON TABLE contact_tags IS '工作区自定义的联系人标签';
COMMENT ON COLUMN contact_tags.id IS '标签编号';
COMMENT ON COLUMN contact_tags.created_at IS '创建时间';
COMMENT ON COLUMN contact_tags.updated_at IS '更新时间';
COMMENT ON COLUMN contact_tags.organization_id IS '所属工作区编号';
COMMENT ON COLUMN contact_tags.name IS '标签名称，工作区内唯一';

-- +goose Down
DROP TABLE contact_tags;
