-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
    ADD COLUMN gender        TEXT NOT NULL DEFAULT '',
    ADD COLUMN birth_date    DATE,
    ADD COLUMN city          TEXT NOT NULL DEFAULT '',
    ADD COLUMN website       TEXT NOT NULL DEFAULT '',
    ADD COLUMN activity      TEXT NOT NULL DEFAULT '',
    ADD COLUMN quote         TEXT NOT NULL DEFAULT '',
    ADD COLUMN bio           TEXT NOT NULL DEFAULT '',
    ADD COLUMN avatar_path   TEXT NOT NULL DEFAULT '',
    ADD COLUMN last_seen_at  TIMESTAMPTZ;

CREATE TYPE profile_scope AS ENUM ('everyone', 'friends', 'nobody');

CREATE TABLE profile_privacy (
    user_id        BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    online_scope   profile_scope NOT NULL DEFAULT 'everyone',
    basic_scope    profile_scope NOT NULL DEFAULT 'everyone',
    friends_scope  profile_scope NOT NULL DEFAULT 'everyone',
    bio_scope      profile_scope NOT NULL DEFAULT 'everyone',
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO profile_privacy (user_id)
SELECT id FROM users
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS profile_privacy;
DROP TYPE  IF EXISTS profile_scope;
ALTER TABLE users
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS avatar_path,
    DROP COLUMN IF EXISTS bio,
    DROP COLUMN IF EXISTS quote,
    DROP COLUMN IF EXISTS activity,
    DROP COLUMN IF EXISTS website,
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS birth_date,
    DROP COLUMN IF EXISTS gender;
-- +goose StatementEnd
