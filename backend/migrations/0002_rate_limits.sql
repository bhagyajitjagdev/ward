-- +goose Up
CREATE TABLE rate_limits (
    id         TEXT PRIMARY KEY,
    scope      TEXT NOT NULL DEFAULT 'global',
    service_id TEXT REFERENCES services(id) ON DELETE CASCADE,
    max_events INTEGER NOT NULL,
    window_dur TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE rate_limits;
