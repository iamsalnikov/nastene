-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS schema_sentinel (
    id         SMALLINT PRIMARY KEY DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT schema_sentinel_single CHECK (id = 1)
);

INSERT INTO schema_sentinel (id) VALUES (1) ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS schema_sentinel;
-- +goose StatementEnd
