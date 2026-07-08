-- +goose Up
CREATE TABLE geo_rules (
    id         TEXT PRIMARY KEY,
    scope      TEXT NOT NULL DEFAULT 'global',
    service_id TEXT REFERENCES services(id) ON DELETE CASCADE,
    countries  TEXT NOT NULL, -- JSON array of ISO 3166-1 alpha-2 country codes to block
    created_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE geo_rules;
