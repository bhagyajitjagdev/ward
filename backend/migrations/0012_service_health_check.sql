-- +goose Up
-- Per-service active health-check config (opt-in): a JSON blob of {active, path,
-- interval, timeout, expect_status}. Passive checks are always emitted by config-gen;
-- active checks add a periodic probe when enabled. Dialect-portable ADD COLUMN.
ALTER TABLE services ADD COLUMN health_check TEXT NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE services DROP COLUMN health_check;
