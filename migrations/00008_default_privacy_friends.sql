-- +goose Up
-- +goose StatementBegin
ALTER TABLE wall_privacy
    ALTER COLUMN view_scope SET DEFAULT 'friends',
    ALTER COLUMN post_scope SET DEFAULT 'friends';

ALTER TABLE profile_privacy
    ALTER COLUMN online_scope  SET DEFAULT 'friends',
    ALTER COLUMN basic_scope   SET DEFAULT 'friends',
    ALTER COLUMN friends_scope SET DEFAULT 'friends',
    ALTER COLUMN bio_scope     SET DEFAULT 'friends';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE wall_privacy
    ALTER COLUMN view_scope SET DEFAULT 'public',
    ALTER COLUMN post_scope SET DEFAULT 'public';

ALTER TABLE profile_privacy
    ALTER COLUMN online_scope  SET DEFAULT 'everyone',
    ALTER COLUMN basic_scope   SET DEFAULT 'everyone',
    ALTER COLUMN friends_scope SET DEFAULT 'everyone',
    ALTER COLUMN bio_scope     SET DEFAULT 'everyone';
-- +goose StatementEnd
