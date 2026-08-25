-- +goose Up
-- 将联系人创建用户编号改为可空。
ALTER TABLE contacts
    ALTER COLUMN created_by_user_id DROP NOT NULL;

COMMENT ON COLUMN contacts.created_by_user_id
    IS '创建用户编号，渠道自动创建时为空';

-- +goose Down
ALTER TABLE contacts
    ALTER COLUMN created_by_user_id SET NOT NULL;

COMMENT ON COLUMN contacts.created_by_user_id
    IS '创建人编号';
