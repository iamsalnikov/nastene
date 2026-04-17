-- +goose Up
-- +goose StatementBegin
ALTER TYPE wall_scope ADD VALUE IF NOT EXISTS 'nobody';

ALTER TABLE wall_privacy
    ADD COLUMN comment_scope wall_scope NOT NULL DEFAULT 'friends';

UPDATE wall_privacy SET comment_scope = post_scope;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE wall_privacy DROP COLUMN comment_scope;
-- +goose StatementEnd
