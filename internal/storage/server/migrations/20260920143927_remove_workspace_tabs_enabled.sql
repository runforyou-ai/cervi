-- +goose Up
-- 工作台不再提供多标签页，移除对应偏好字段。
ALTER TABLE users DROP COLUMN workspace_tabs_enabled;

-- +goose Down
ALTER TABLE users ADD COLUMN workspace_tabs_enabled boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN users.workspace_tabs_enabled IS '是否启用工作台多标签页';
