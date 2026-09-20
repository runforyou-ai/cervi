-- +goose Up
-- 创建成员本机设备表。
CREATE TABLE devices (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    organization_id uuid NOT NULL,
    user_id         uuid NOT NULL,
    install_id      text NOT NULL,
    name            text NOT NULL,
    platform        text NOT NULL,
    revoked_at      timestamptz
);

CREATE UNIQUE INDEX devices_organization_user_install_unique
    ON devices (organization_id, user_id, install_id);

COMMENT ON TABLE devices IS '成员注册到企业的本机设备';
COMMENT ON COLUMN devices.id IS '设备编号';
COMMENT ON COLUMN devices.created_at IS '注册时间';
COMMENT ON COLUMN devices.updated_at IS '更新时间';
COMMENT ON COLUMN devices.organization_id IS '所属企业编号';
COMMENT ON COLUMN devices.user_id IS '设备主人编号';
COMMENT ON COLUMN devices.install_id IS '客户端安装标识，同一安装重复注册指向同一设备';
COMMENT ON COLUMN devices.name IS '设备名称';
COMMENT ON COLUMN devices.platform IS '设备平台';
COMMENT ON COLUMN devices.revoked_at IS '撤销时间，非空表示该设备当前不受信任';

-- +goose Down
DROP TABLE devices;
