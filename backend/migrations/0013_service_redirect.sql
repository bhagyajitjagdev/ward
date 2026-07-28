-- +goose Up
-- Redirect-only services: a JSON blob of {to, status, preserve_path}. When `to` is set
-- the service emits a 301/302 redirect instead of a reverse proxy (no upstreams needed).
-- Dialect-portable ADD COLUMN.
ALTER TABLE services ADD COLUMN redirect TEXT NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE services DROP COLUMN redirect;
