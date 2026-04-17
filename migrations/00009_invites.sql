-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
    ADD COLUMN invites_remaining  INT    NOT NULL DEFAULT 2,
    ADD COLUMN invite_token       UUID   NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN invited_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX users_invite_token_idx ON users(invite_token);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS users_invite_token_idx;
ALTER TABLE users
    DROP COLUMN IF EXISTS invited_by_user_id,
    DROP COLUMN IF EXISTS invite_token,
    DROP COLUMN IF EXISTS invites_remaining;
-- +goose StatementEnd
