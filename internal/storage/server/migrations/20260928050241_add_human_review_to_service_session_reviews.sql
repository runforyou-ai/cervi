-- +goose Up
-- 周期质检增加真人客服的答错与态度结论。
ALTER TABLE service_session_reviews
    ADD COLUMN human_incorrect boolean,
    ADD COLUMN human_poor_attitude boolean;

COMMENT ON TABLE service_session_reviews IS '客服周期质检：判断模型在周期关闭后给出的满意度与 AI 客服、真人客服质检结论';
COMMENT ON COLUMN service_session_reviews.human_incorrect IS '真人客服答复有误；周期内没有真人对客回复时为空';
COMMENT ON COLUMN service_session_reviews.human_poor_attitude IS '真人客服态度问题；周期内没有真人对客回复时为空';

-- +goose Down
ALTER TABLE service_session_reviews
    DROP COLUMN human_incorrect,
    DROP COLUMN human_poor_attitude;

COMMENT ON TABLE service_session_reviews IS '客服周期质检：判断模型在周期关闭后给出的满意度与 AI 客服质检结论';
