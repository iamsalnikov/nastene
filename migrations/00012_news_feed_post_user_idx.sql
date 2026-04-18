-- +goose Up
DROP INDEX news_feed_post_idx;
CREATE INDEX news_feed_post_user_idx ON news_feed (post_id, user_id);

-- +goose Down
DROP INDEX news_feed_post_user_idx;
CREATE INDEX news_feed_post_idx ON news_feed (post_id);
