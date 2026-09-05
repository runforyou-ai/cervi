-- +goose Up
CREATE TABLE conversation_mention_reviews (
    organization_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    user_id uuid NOT NULL,
    message_id uuid NOT NULL,
    PRIMARY KEY (organization_id, conversation_id, user_id, message_id)
);

COMMENT ON TABLE conversation_mention_reviews IS '连续查看水位之后已单独查看的群聊提及';
COMMENT ON COLUMN conversation_mention_reviews.organization_id IS '所属企业编号';
COMMENT ON COLUMN conversation_mention_reviews.conversation_id IS '群聊编号';
COMMENT ON COLUMN conversation_mention_reviews.user_id IS '查看用户编号';
COMMENT ON COLUMN conversation_mention_reviews.message_id IS '已查看提及消息编号';

-- +goose Down
DROP TABLE conversation_mention_reviews;
