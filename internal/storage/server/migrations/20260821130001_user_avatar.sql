-- +goose Up
-- 为企业成员保存头像文件关联。
ALTER TABLE users
    ADD COLUMN avatar_file_id uuid;

COMMENT ON COLUMN users.avatar_file_id IS '当前头像文件编号';

-- +goose Down
ALTER TABLE users
    DROP COLUMN avatar_file_id;
