-- +goose Up
COMMENT ON COLUMN messages.visibility IS '消息可见范围：shared 会话各方可见，internal 仅服务会话的处理方可见，requester 仅企业成员发起人可见';

-- +goose Down
COMMENT ON COLUMN messages.visibility IS '消息可见范围：shared 会话各方可见，internal 仅服务会话的处理方可见';
