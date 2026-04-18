-- +goose Up
CREATE TABLE news_feed (
    id          BIGSERIAL   PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id)      ON DELETE CASCADE,
    kind        TEXT        NOT NULL,
    post_id     BIGINT      NOT NULL REFERENCES wall_posts(id) ON DELETE CASCADE,
    comment_id  BIGINT               REFERENCES comments(id)   ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL,
    hidden_at   TIMESTAMPTZ,
    CONSTRAINT news_feed_kind_chk CHECK (kind IN ('post', 'comment')),
    CONSTRAINT news_feed_comment_chk CHECK (
        (kind = 'post'    AND comment_id IS NULL) OR
        (kind = 'comment' AND comment_id IS NOT NULL)
    )
);

CREATE INDEX news_feed_user_time_idx
    ON news_feed (user_id, created_at DESC, id DESC)
    WHERE hidden_at IS NULL;

CREATE INDEX news_feed_post_idx ON news_feed (post_id);

CREATE UNIQUE INDEX news_feed_uniq_post
    ON news_feed (user_id, post_id)
    WHERE kind = 'post';

CREATE UNIQUE INDEX news_feed_uniq_comment
    ON news_feed (user_id, comment_id)
    WHERE kind = 'comment';

-- +goose Down
DROP TABLE news_feed;
