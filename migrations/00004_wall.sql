-- +goose Up
-- +goose StatementBegin
CREATE TYPE wall_scope AS ENUM ('public', 'friends');

CREATE TABLE wall_privacy (
    user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    view_scope   wall_scope NOT NULL DEFAULT 'public',
    post_scope   wall_scope NOT NULL DEFAULT 'public',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TYPE wall_post_kind AS ENUM ('text', 'graffiti');

CREATE TABLE wall_posts (
    id             BIGSERIAL PRIMARY KEY,
    wall_owner_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    author_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind           wall_post_kind NOT NULL,
    body_text      TEXT NOT NULL DEFAULT '',
    graffiti_path  TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT wall_posts_kind_body CHECK (
        (kind = 'text' AND body_text <> '' AND graffiti_path = '') OR
        (kind = 'graffiti' AND graffiti_path <> '')
    )
);

CREATE INDEX wall_posts_owner_created_idx ON wall_posts (wall_owner_id, created_at DESC);

CREATE TABLE bans (
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    banned_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, banned_id),
    CONSTRAINT bans_no_self CHECK (user_id <> banned_id)
);

CREATE INDEX bans_banned_idx ON bans (banned_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS bans;
DROP TABLE IF EXISTS wall_posts;
DROP TABLE IF EXISTS wall_privacy;
DROP TYPE IF EXISTS wall_post_kind;
DROP TYPE IF EXISTS wall_scope;
-- +goose StatementEnd
