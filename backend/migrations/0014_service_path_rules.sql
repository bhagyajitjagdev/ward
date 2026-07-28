-- +goose Up
-- Path-based routing: a JSON array of {path, match, upstreams, deny, strip_prefix}. Each
-- rule routes one path of the host to its own backend (or denies it with a 403); the
-- service's own upstreams stay the default (catch-all). Empty array ⇒ single-pool as before.
-- Dialect-portable ADD COLUMN.
ALTER TABLE services ADD COLUMN path_rules TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE services DROP COLUMN path_rules;
