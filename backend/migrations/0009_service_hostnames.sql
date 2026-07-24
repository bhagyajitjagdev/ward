-- +goose Up
-- Multi-hostname services. The primary name stays in services.public_hostname
-- (kept UNIQUE, DB-enforced); additional names live here as a JSON array. The full
-- list a service serves is [public_hostname, ...extra_hostnames]. Cross-service
-- uniqueness of the extras is enforced in the app layer (no portable DB constraint
-- across a scalar column + a JSON list). ADD/DROP COLUMN only — dialect-portable.
ALTER TABLE services ADD COLUMN extra_hostnames TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE services DROP COLUMN extra_hostnames;
