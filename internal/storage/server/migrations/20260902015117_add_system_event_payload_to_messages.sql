-- +goose Up
ALTER TABLE messages
    ADD COLUMN system_event_type text,
    ADD COLUMN system_event_payload jsonb;

COMMENT ON COLUMN messages.system_event_type IS '系统事件类型';
COMMENT ON COLUMN messages.system_event_payload IS '系统事件的类型化审计载荷';

-- +goose Down
ALTER TABLE messages
    DROP COLUMN system_event_payload,
    DROP COLUMN system_event_type;
