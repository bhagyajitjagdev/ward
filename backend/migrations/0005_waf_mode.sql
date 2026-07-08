-- +goose Up
-- Per-service WAF engine-mode override: '' = inherit the global default (settings
-- key 'waf.engine_mode'); otherwise 'DetectionOnly' or 'On'.
ALTER TABLE services ADD COLUMN waf_mode TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE services DROP COLUMN waf_mode;
