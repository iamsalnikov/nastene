-- +goose Up
-- +goose StatementBegin
CREATE TABLE friend_requests (
    from_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    to_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (from_id, to_id),
    CONSTRAINT friend_requests_no_self CHECK (from_id <> to_id)
);

CREATE INDEX friend_requests_to_idx ON friend_requests (to_id);

CREATE TABLE friendships (
    user_a      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_a, user_b),
    CONSTRAINT friendships_ordered CHECK (user_a < user_b)
);

CREATE INDEX friendships_user_b_idx ON friendships (user_b);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS friendships;
DROP TABLE IF EXISTS friend_requests;
-- +goose StatementEnd
